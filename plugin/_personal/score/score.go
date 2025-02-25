// Package score 签到系统
package score

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	fcext "github.com/FloatTech/floatbox/ctxext"
	ctrl "github.com/FloatTech/zbpctrl"
	control "github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/disintegration/imaging"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"

	// 数据库

	"github.com/FloatTech/AnimeAPI/wallet"
	sql "github.com/FloatTech/sqlite"

	// 图片输出
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/web"
	"github.com/FloatTech/gg"
	"github.com/FloatTech/imgfactory"
	"github.com/FloatTech/zbputils/img/text"
)

type score struct {
	db sql.Sqlite
	sync.RWMutex
}

// 用户数据信息
type userdata struct {
	Uid        int64  // `Userid`
	UserName   string // `User`
	UpdatedAt  int64  // `签到时间`
	Continuous int    // `连续签到次数`
	Level      int    // `决斗者等级`
	Picname    string // `签到图片`
}

const scoreMax = 1415

var (
	/************************************10*****20******30*****40*****50******60*****70*****80******90**************/
	/*************************2******10*****20******40*****70*****110******160******220***290*****370*******460***************/
	levelrank = [...]string{"新手", "青铜", "白银", "黄金", "白金Ⅲ", "白金Ⅱ", "白金Ⅰ", "传奇Ⅲ", "传奇Ⅱ", "传奇Ⅰ", "决斗王"}
	engine    = control.Register("score", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault:  false,
		Brief:             "签到",
		PrivateDataFolder: "ygoscore",
		Help:              "- 签到\n- 获得签到背景",
	})
	cachePath = engine.DataFolder() + "cache/"
	mu        sync.RWMutex
	dbpath    = engine.DataFolder() + "score.db"
	scoredata = &score{db: sql.New(dbpath)}
)

func init() {
	go func() {
		err := os.MkdirAll(cachePath, 0755)
		if err != nil {
			panic(err)
		}
	}()
	getdb := fcext.DoOnceOnSuccess(func(ctx *zero.Ctx) bool {
		err := scoredata.db.Open(time.Hour * 24)
		if err != nil {
			ctx.SendChain(message.Text("[init ERROR]:", err))
			return false
		}
		err = scoredata.db.Create("score", &userdata{})
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return false
		}
		return true
	})

	engine.OnFullMatchGroup([]string{"签到", "打卡"}, getdb).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		uid := ctx.Event.UserID
		userinfo := scoredata.getData(uid)
		userinfo.Uid = uid
		userinfo.UserName = ctx.CardOrNickName(uid) // 更新昵称
		lasttime := time.Unix(userinfo.UpdatedAt, 0)
		score := wallet.GetWalletOf(uid)
		// 判断是否已经签到过了
		if time.Now().Format("2006/01/02") == lasttime.Format("2006/01/02") {
			if userinfo.Picname == "" {
				picFile, err := initPic()
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				if picFile != "" {
					userinfo.Picname = picFile
					if err := scoredata.setData(userinfo); err != nil {
						ctx.SendChain(message.Text("[ERROR]:签到记录失败。", err))
						return
					}
				}
			}
			data, err := drawimagePro(&userinfo, score, 0)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			ctx.SendChain(message.Text("今天已经签到过了"))
			ctx.SendChain(message.ImageBytes(data))
			return
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			picFile, err := initPic()
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			if picFile == "" {
				return
			}
			userinfo.Picname = picFile
			if err := scoredata.setData(userinfo); err != nil {
				ctx.SendChain(message.Text("[ERROR]:签到记录失败。", err))
				return
			}
		}()
		add := 1
		wg.Add(1)
		go func() {
			// 更新数据
			subtime := time.Since(lasttime).Hours()
			if subtime > 48 {
				userinfo.Continuous = 1
			} else {
				userinfo.Continuous += 1
				add = int(math.Min(5, float64(userinfo.Continuous)))
			}
			userinfo.UpdatedAt = time.Now().Unix()
			if userinfo.Level < scoreMax {
				userinfo.Level += add
			}
			defer wg.Done()
			if err := scoredata.setData(userinfo); err != nil {
				ctx.SendChain(message.Text("[ERROR]:签到记录失败。", err))
				return
			}
			level, _ := getLevel(userinfo.Level)
			if err := wallet.InsertWalletOf(uid, add+level*5); err != nil {
				ctx.SendChain(message.Text("[ERROR]:货币记录失败。", err))
				return
			}
			score = wallet.GetWalletOf(uid)
		}()
		// 生成签到图片
		wg.Wait()
		data, err := drawimagePro(&userinfo, score, add)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		ctx.SendChain(message.ImageBytes(data))
	})
	engine.OnPrefix("获得签到背景").Limit(ctxext.LimitByGroup).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			uid := ctx.Event.UserID
			if len(ctx.Event.Message) > 1 && ctx.Event.Message[1].Type == "at" {
				uid, _ = strconv.ParseInt(ctx.Event.Message[1].Data["qq"], 10, 64)
			}
			userinfo := scoredata.getData(uid)
			picFile := userinfo.Picname
			if file.IsNotExist(picFile) {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("请先签到！"))
				return
			}
			// ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + picFile))

			pic, err := os.ReadFile(picFile)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]", err))
				return
			}
			ctx.SendChain(message.ImageBytes(pic))
		})
	engine.OnRegex(`^\/修改(\s*(\[CQ:at,qq=)?(\d+).*)?信息\s*(.*)`, zero.AdminPermission, getdb).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		changeuser := ctx.State["regex_matched"].([]string)[3]
		data := ctx.State["regex_matched"].([]string)[4]
		uid := ctx.Event.UserID
		changeData := make(map[string]string, 10)
		infoList := strings.Split(data, " ")
		if len(infoList) == 1 {
			ctx.SendChain(message.Text("[ERROR]:", "请输入正确的参数"))
			return
		}
		for _, manager := range infoList {
			infoData := strings.Split(manager, ":")
			if len(infoData) > 1 {
				changeData[infoData[0]] = infoData[1]
			}
		}
		if changeuser != "" {
			uid, _ = strconv.ParseInt(changeuser, 10, 64)
		}
		userinfo := scoredata.getData(uid)
		userinfo.Uid = uid
		for dataName, value := range changeData {
			switch dataName {
			case "签到时间":
				now, err := time.Parse("2006/01/02", value)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				userinfo.UpdatedAt = now.Unix()
			case "签到次数":
				times, err := strconv.Atoi(value)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				userinfo.Continuous = times
			case "等级":
				level, err := strconv.Atoi(value)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				userinfo.Level = level
			}
		}
		err := scoredata.db.Insert("score", &userinfo)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		ctx.SendChain(message.Text("成功"))
	})
}

// 获取签到数据
func (sdb *score) getData(uid int64) (userinfo userdata) {
	sdb.Lock()
	defer sdb.Unlock()
	_ = sdb.db.Find("score", &userinfo, "where uid = "+strconv.FormatInt(uid, 10))
	return
}

// 保存签到数据
func (sdb *score) setData(userinfo userdata) error {
	sdb.Lock()
	defer sdb.Unlock()
	return sdb.db.Insert("score", &userinfo)

}

// DownloadTo 下载到路径
func DownloadTo(url, path string) (filePath string, err error) {
	mu.Lock()
	defer mu.Unlock()
	resp, err := http.Get(url)
	if err == nil {
		var f *os.File
		info, err1 := os.Stat(path)
		if err1 != nil {
			err = err1
			return
		}
		if info.IsDir() {
			if file.IsNotExist(path) {
				err = os.Mkdir(path, 0755)
				if err != nil {
					return
				}
			}
			fileInfo := resp.Header["Content-Disposition"][0]
			fileName := strings.Replace(fileInfo[strings.LastIndex(fileInfo, "=")+1:], "\"", "", -1)
			filePath = filepath.Join(path, fileName)
		} else {
			filePath = path
		}
		f, err = os.Create(filePath)
		if err == nil {
			_, err = io.Copy(f, resp.Body)
			f.Close()
		}
		resp.Body.Close()
	}
	return
}

// 下载图片
func initPic() (picFile string, err error) {
	picFile, err = DownloadTo("https://img.moehu.org/pic.php", cachePath)
	if err != nil {
		fmt.Println("[score] 下载图片失败,将从下载其他二次元图片:", err)
		return otherPic()
	}
	return
}

// 下载图片
func otherPic() (picFile string, err error) {
	apiList := []string{"http://81.70.100.130/api/DmImg.php", "http://81.70.100.130/api/acgimg.php"}
	picFile, err = DownloadTo(apiList[rand.Intn(len(apiList))], cachePath)
	if err != nil {
		fmt.Println("[score] 下载图片失败,将从本地抽取:", err)
		return randFile(3)
	}
	return
}

func randFile(indexMax int) (string, error) {
	files, err := os.ReadDir(file.BOTPATH + cachePath)
	if err != nil {
		return "", err
	}
	if len(files) > 0 {
		drawFile := files[rand.Intn(len(files))].Name()
		// 如果是文件夹就递归
		before, _, ok := strings.Cut(drawFile, ".")
		if !ok || before == "" {
			indexMax--
			if indexMax <= 0 {
				return "", errors.New("存在太多非图片文件,请清理~")
			}
			return randFile(indexMax)
		}
		return cachePath + drawFile, err
	}
	return "", errors.New("不存在本地签到图片")
}

func drawimagePro(userinfo *userdata, score, add int) (data []byte, err error) {
	if userinfo.Picname == "" {
		err = errors.New("[ERROR]:签到图片获取失败")
		return
	}
	back, err := gg.LoadImage(userinfo.Picname)
	if err != nil {
		return
	}
	imgDX := back.Bounds().Dx()
	imgDY := back.Bounds().Dy()
	backDX := 1500

	imgDW := backDX - 100
	scale := float64(imgDW) / float64(imgDX)
	imgDH := int(float64(imgDY) * scale)
	back = imgfactory.Size(back, imgDW, imgDH).Image()

	backDY := imgDH + 500
	canvas := gg.NewContext(backDX, backDY)
	// 放置毛玻璃背景
	backBlurW := float64(imgDW) * (float64(backDY) / float64(imgDH))
	canvas.DrawImageAnchored(imaging.Blur(imgfactory.Size(back, int(backBlurW), backDY).Image(), 8), backDX/2, backDY/2, 0.5, 0.5)
	canvas.DrawRectangle(1, 1, float64(backDX), float64(backDY))
	canvas.SetLineWidth(3)
	canvas.SetRGBA255(255, 255, 255, 100)
	canvas.StrokePreserve()
	canvas.SetRGBA255(255, 255, 255, 140)
	canvas.Fill()
	// 信息框
	canvas.DrawRoundedRectangle(20, 20, 1500-20-20, 450-20, (450-20)/5)
	canvas.SetLineWidth(6)
	canvas.SetDash(20.0, 10.0, 0)
	canvas.SetRGBA255(255, 255, 255, 255)
	canvas.Stroke()
	// 放置头像
	getAvatar, err := web.GetData("http://q4.qlogo.cn/g?b=qq&nk=" + strconv.FormatInt(userinfo.Uid, 10) + "&s=640")
	if err != nil {
		return
	}
	avatar, _, err := image.Decode(bytes.NewReader(getAvatar))
	if err != nil {
		return
	}
	avatarf := imgfactory.Size(avatar, 270, 270)
	canvas.DrawCircle(50+float64(avatarf.W())/2, 50+float64(avatarf.H())/2, float64(avatarf.W())/2+2)
	canvas.SetLineWidth(3)
	canvas.SetDash()
	canvas.SetRGBA255(255, 255, 255, 255)
	canvas.Stroke()
	canvas.DrawImage(avatarf.Circle(0).Image(), 50, 50)
	// 放置昵称
	canvas.SetRGB(0, 0, 0)
	fontSize := 150.0
	data, err = file.GetLazyData(text.BoldFontFile, control.Md5File, true)
	if err != nil {
		return
	}
	if err = canvas.ParseFontFace(data, fontSize); err != nil {
		return
	}
	nameW, nameH := canvas.MeasureString(userinfo.UserName)
	// 昵称范围
	textH := 300.0
	textW := float64(backDX) * 2 / 3
	// 如果文字超过长度了，比列缩小字体
	if nameW > textW {
		scale := 2 * nameH / textH
		fontSize = fontSize * scale
		if err = canvas.ParseFontFace(data, fontSize); err != nil {
			return
		}
		_, nameH := canvas.MeasureString(userinfo.UserName)
		// 昵称分段
		name := []rune(userinfo.UserName)
		names := make([]string, 0, 4)
		// 如果一半都没到界面边界就分两行
		wordw, _ := canvas.MeasureString(string(name[:len(name)/2]))
		if wordw < textW*3/4 {
			names = append(names, string(name[:len(name)/2+1]))
			names = append(names, string(name[len(name)/2+1:]))
		} else {
			nameLength := 0.0
			lastIndex := 0
			for i, word := range name {
				wordw, _ = canvas.MeasureString(string(word))
				nameLength += wordw
				if nameLength > textW*3/4 || i == len(name)-1 {
					names = append(names, string(name[lastIndex:i+1]))
					lastIndex = i + 1
					nameLength = 0
				}
			}
			// 超过两行就重新配置一下字体大小
			scale = float64(len(names)) * nameH / textH
			fontSize = fontSize * scale
			if err = canvas.ParseFontFace(data, fontSize); err != nil {
				return
			}
		}
		for i, nameSplit := range names {
			canvas.DrawStringAnchored(nameSplit, float64(backDX)/2+25, 25+(200+70*scale)*float64(i+1)/float64(len(names))-nameH/2, 0.5, 0.5)
		}
	} else {
		canvas.DrawStringAnchored(userinfo.UserName, float64(backDX)/2+25, 200-nameH/2, 0.5, 0.5)
	}

	// level
	if err = canvas.ParseFontFace(data, 72); err != nil {
		return
	}
	level, nextLevelScore := getLevel(userinfo.Level)
	if level == -1 {
		err = errors.New("计算等级出现了问题")
		return
	}
	rank := levelrank[level]
	textW, textH = canvas.MeasureString(rank)
	levelX := float64(backDX) * 4 / 5
	canvas.DrawRoundedRectangle(levelX, 50, textW*1.2, 200, 200/5)
	canvas.SetLineWidth(3)
	canvas.SetRGBA255(0, 0, 0, 100)
	canvas.StrokePreserve()
	canvas.SetRGBA255(255, 255, 255, 100)
	canvas.Fill()
	canvas.DrawRoundedRectangle(levelX, 50, textW*1.2, 100, 200/5)
	canvas.SetLineWidth(3)
	canvas.SetRGBA255(0, 0, 0, 100)
	canvas.StrokePreserve()
	canvas.SetRGBA255(255, 255, 255, 100)
	canvas.Fill()
	canvas.SetRGBA255(0, 0, 0, 255)
	canvas.DrawStringAnchored(levelrank[level], levelX+textW*1.2/2, 50+50, 0.5, 0.5)
	canvas.DrawStringAnchored(fmt.Sprintf("LV%d", level), levelX+textW*1.2/2, 50+100+50, 0.5, 0.5)

	if add == 0 {
		canvas.DrawStringAnchored(fmt.Sprintf("已连签 %d 天    总资产: %d", userinfo.Continuous, score), float64(backDX)/2+100, 370-textH/2, 0.5, 0.5)
	} else {
		canvas.DrawStringAnchored(fmt.Sprintf("连签 %d 天 总资产(+%d): %d", userinfo.Continuous, add+level*5, score), float64(backDX)/2+100, 370-textH/2, 0.5, 0.5)
	}
	// 绘制等级进度条
	if err = canvas.ParseFontFace(data, 50); err != nil {
		return
	}
	_, textH = canvas.MeasureString("/")
	switch {
	case userinfo.Level < scoreMax && add == 0:
		canvas.DrawStringAnchored(fmt.Sprintf("%d/%d", userinfo.Level, nextLevelScore), float64(backDX)/2, 455-textH, 0.5, 0.5)
	case userinfo.Level < scoreMax:
		canvas.DrawStringAnchored(fmt.Sprintf("(%d+%d)/%d", userinfo.Level-add, add, nextLevelScore), float64(backDX)/2, 455-textH, 0.5, 0.5)
	default:
		canvas.DrawStringAnchored("Max/Max", float64(backDX)/2, 455-textH, 0.5, 0.5)

	}
	// 创建彩虹条
	grad := gg.NewLinearGradient(0, 500, 1500, 300)
	grad.AddColorStop(0, color.RGBA{G: 255, A: 255})
	grad.AddColorStop(0.35, color.RGBA{B: 255, A: 255})
	grad.AddColorStop(0.5, color.RGBA{R: 255, A: 255})
	grad.AddColorStop(0.65, color.RGBA{B: 255, A: 255})
	grad.AddColorStop(1, color.RGBA{G: 255, A: 255})
	canvas.SetStrokeStyle(grad)
	canvas.SetLineWidth(7)
	// 设置长度
	gradMax := 1300.0
	LevelLength := gradMax * (float64(userinfo.Level) / float64(nextLevelScore))
	canvas.MoveTo((float64(backDX)-LevelLength)/2, 450)
	canvas.LineTo((float64(backDX)+LevelLength)/2, 450)
	canvas.ClosePath()
	canvas.Stroke()
	// 放置图片
	canvas.DrawImageAnchored(back, backDX/2, imgDH/2+475, 0.5, 0.5)
	// 生成图片
	return imgfactory.ToBytes(canvas.Image())
}
func getLevel(count int) (int, int) {
	switch {
	case count < 5:
		return 0, 5
	case count > scoreMax:
		return len(levelrank) - 1, scoreMax
	default:
		i := 10
		for k := 1; k < len(levelrank); i += (k * 10) * scoreMax / 460 {
			if count < i {
				return k, i
			}
			k++
		}
		return len(levelrank) - 1, scoreMax
	}
}
