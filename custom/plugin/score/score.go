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
	"net/url"
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
	trshttp "github.com/fumiama/terasu/http"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
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

	"golang.org/x/image/webp"
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

var (
	levelrank = [...]string{
		"新手",  // 0-4
		"入门",  // 5-9
		"青铜Ⅲ", // 10-14
		"青铜Ⅱ", // 15-19
		"青铜Ⅰ", // 20-24
		"白银Ⅲ", // 25-29
		"白银Ⅱ", // 30-24
		"白银Ⅰ", // 35-39
		"黄金Ⅲ", // 40-44
		"黄金Ⅱ", // 45-49
		"黄金Ⅰ", // 50-54
		"白金Ⅲ", // 55-59
		"白金Ⅱ", // 60-64
		"白金Ⅰ", // 65-69
		"钻石Ⅲ", // 70-74
		"钻石Ⅱ", // 75-79
		"钻石Ⅰ", // 80-84
		"传奇Ⅲ", // 85-89
		"传奇Ⅱ", // 90-94
		"传奇Ⅰ", // 95-99
		"决斗王", // 100
	}
	engine = control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault:  false,
		Brief:             "签到",
		PrivateDataFolder: "score",
		Help:              "- 签到\n- 获得签到背景",
	}).ApplySingle(ctxext.DefaultSingle)
	cachePath      = engine.DataFolder() + "ygo/"
	cacheOtherPath = engine.DataFolder() + "cache/"
	mu             sync.RWMutex
	dbpath         = engine.DataFolder() + "score.db"
	scoredata      = &score{db: sql.New(dbpath)}
)

func init() {
	go func() {
		err := os.MkdirAll(cachePath, 0755)
		if err != nil {
			panic(err)
		}
		err = os.MkdirAll(cacheOtherPath, 0755)
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

	engine.OnFullMatch("/hso").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		imgurl, err := getimgurl("https://api.lolicon.app/setu/v2?tag=" + url.QueryEscape("游戏王|yu-gi-oh|遊戯王"))
		if err != nil {
			fmt.Println(err)
			return
		}
		imgPath, err := DownloadTo(imgurl, file.BOTPATH+"/"+cachePath)
		if err != nil {
			fmt.Println(err)
			return
		}
		// ctx.SendChain(message.Image("file://" + imgPath))
		pic, err := os.ReadFile(imgPath)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]", err))
			return
		}
		ctx.SendChain(message.ImageBytes(pic))
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
				picFile, err := initPic(0)
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
			} else if file.IsNotExist(userinfo.Picname) {
				picFile, err := randFile(cachePath, 0)
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
			data, err := drawImage(&userinfo, score, 0)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			ctx.SendChain(message.Text("今天已经签到过了"))
			ctx.SendChain(message.ImageBytes(data))
			return
		}
		go func() {
			_, err := initPic(0)
			if err != nil {
				logrus.Debugln("[score] 初始化签到图片失败:", err)
				return
			}
		}()
		// sudu := rand.Intn(100)
		var wg sync.WaitGroup
		var syncerr error = nil
		wg.Add(1)
		go func() {
			defer wg.Done()
			// ctx.SendChain(message.Text("「debug」正在以龟速(", sudu, "kb/s)获取签到背景..."))
			picFile, err := randFile(cachePath, 0)
			if err != nil {
				syncerr = err
				return
			}
			if picFile == "" {
				syncerr = errors.New("[ERROR]:没有可用的签到图片")
				return
			}
			userinfo.Picname = picFile
			if err := scoredata.setData(userinfo); err != nil {
				syncerr = fmt.Errorf("[ERROR]:签到记录失败。%w", err)
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
			rankIndex, level, _ := getLevel(userinfo.Level)
			if rankIndex*5+level < 101 {
				userinfo.Level += add
			}
			defer wg.Done()
			if err := scoredata.setData(userinfo); err != nil {
				syncerr = fmt.Errorf("[ERROR]:更新签到数据失败。%w", err)
				return
			}
			if err := wallet.InsertWalletOf(uid, add+rankIndex*5); err != nil {
				syncerr = fmt.Errorf("[ERROR]:更新钱包失败。%w", err)
				return
			}
			score = wallet.GetWalletOf(uid)
		}()
		// 生成签到图片
		wg.Wait()
		if syncerr != nil {
			ctx.SendChain(message.Text(syncerr.Error()))
			return
		}
		data, err := drawImage(&userinfo, score, add)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		ctx.SendChain(message.ImageBytes(data))
	})
	engine.OnKeywordGroup([]string{"签到背景", "打卡背景", "签到图片", "打卡图片"}).Limit(ctxext.LimitByGroup).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			uid := ctx.Event.UserID
			score := wallet.GetWalletOf(uid)
			if score < 100 {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("你的", wallet.GetWalletName(), "不足100,无法获取签到背景。"))
				return
			}
			if len(ctx.Event.Message) > 1 && ctx.Event.Message[1].Type == "at" {
				uid, _ = strconv.ParseInt(ctx.Event.Message[1].Data["qq"], 10, 64)
			}
			userinfo := scoredata.getData(uid)
			picFile := userinfo.Picname
			if picFile == "" || file.IsNotExist(picFile) {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("请先签到！"))
				return
			}
			// ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + picFile))
			pic, err := os.ReadFile(picFile)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]", err))
				return
			}
			if msgID := ctx.SendChain(message.ImageBytes(pic)); msgID.ID() != 0 {
				err := wallet.InsertWalletOf(uid, -100)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("已扣除100", wallet.GetWalletName()))
			}
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
func DownloadTo(url, path string) (imagePath string, err error) {
	mu.Lock()
	defer mu.Unlock()
	// resp, err := http.Get(url)
	// if err != nil {
	// 	return "", fmt.Errorf("下载请求失败: %v", err)
	// }
	// defer resp.Body.Close()

	var resp *http.Response
	resp, err = trshttp.Get(url)
	if err != nil {
		fmt.Println("trshttp:", err)
		resp, err = http.Get(url)
	}
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败, HTTP状态码: %d", resp.StatusCode)
	}

	if resp.Body == http.NoBody {
		return
	}

	filename := getFilename(url, resp.Header)

	if file.IsNotExist(path) {
		err = os.Mkdir(path, 0755)
		if err != nil {
			return
		}
	}
	imagePath = filepath.Join(path, filename)
	if file.IsExist(imagePath) {
		return imagePath, nil
	}

	// f, err := os.Create(imagePath)
	// if err != nil {
	// 	return "", fmt.Errorf("创建文件失败: %v", err)
	// }
	// defer f.Close()

	// if _, err = io.Copy(f, resp.Body); err != nil {
	// 	return "", fmt.Errorf("写入文件失败: %v", err)
	// }
	// 获取内容长度（如果可用）
	contentLength := resp.ContentLength
	var downloaded int64

	out, err := os.Create(imagePath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %v", err)
	}
	defer out.Close()

	// 创建带缓冲的写入器
	buf := make([]byte, 32*1024) // 32KB缓冲区
	writer := io.MultiWriter(out)

	for {
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := writer.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			downloaded += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}

		// 显示进度（仅当知道总大小时）
		if contentLength > 0 {
			fmt.Printf("\rDownloading... %d%%\n", int(downloaded*100/contentLength))
		}
	}

	return
}

// 从URL获取文件名
func getFilename(url string, header http.Header) string {
	// 尝试从Content-Disposition头获取文件名
	contentDisposition := header.Get("Content-Disposition")
	if contentDisposition != "" {
		if idx := strings.Index(contentDisposition, "filename="); idx != -1 {
			filename := contentDisposition[idx+len("filename="):]
			filename = strings.Trim(filename, `"`)
			return filename
		}
	}

	// 从URL路径中获取文件名
	_, file := filepath.Split(url)
	return file
}

func initPic(idex int) (picFile string, err error) {
	if idex > 3 {
		fmt.Println("[score] lolicon下载图片失败,将从moehu下载图片:", err)
		return moehu(0)
	}
	imgurl, err := getimgurl("https://api.lolicon.app/setu/v2?tag=" + url.QueryEscape("游戏王|yu-gi-oh|遊戯王"))
	if err != nil {
		fmt.Println(err)
		return initPic(idex + 1)
	}
	fmt.Println("[score] lolicon解析地址:", imgurl)
	picFile, err = DownloadTo(imgurl, cachePath)
	if err != nil {
		fmt.Println(err)
		return initPic(idex + 1)
	}
	fmt.Println("[score] lolicon下载成功:", picFile)
	return

}

func getimgurl(url string) (string, error) {
	data, err := web.GetData(url)
	if err != nil {
		return "", err
	}
	json := gjson.ParseBytes(data)
	if e := json.Get("error").Str; e != "" {
		return "", errors.New(e)
	}
	var imageurl string
	if imageurl = json.Get("data.0.urls.original").Str; imageurl == "" {
		return "", errors.New("未找到相关内容, 换个tag试试吧")
	}
	return imageurl, nil
}

// 下载图片
func moehu(idex int) (picFile string, err error) {
	if idex > 3 {
		fmt.Println("[score] moehu下载图片失败,将从alcy下载图片:", err)
		return otherPic(0)
	}
	picFile, err = DownloadTo("https://img.moehu.org/pic.php", cacheOtherPath)
	if err != nil {
		fmt.Println(err)
		return moehu(idex + 1)
	}
	return
}

// 下载图片
func otherPic(idex int) (picFile string, err error) {
	if idex > 3 {
		fmt.Println("[score] alcy下载图片失败,将从本地抽选:", err)
		return randFile(cachePath, 3)
	}
	resp, err := web.HeadRequestURL("https://t.alcy.cc/ycy/")
	if err != nil {
		fmt.Println(err)
		return otherPic(idex + 1)
	}
	fmt.Println("[score] 新链接:", resp)
	picFile, err = DownloadTo(resp, cacheOtherPath)
	if err != nil {
		fmt.Println(err)
		return otherPic(idex + 1)
	}
	return
}

func randFile(dirPath string, indexMax int) (string, error) {
	fullPath := filepath.Join(file.BOTPATH, dirPath)

	files, err := os.ReadDir(fullPath)
	if err != nil {
		return "", fmt.Errorf("读取目录失败: %w", err)
	}

	if len(files) == 0 {
		return "", errors.New("不存在本地签到图片")
	}

	rand.Shuffle(len(files), func(i, j int) {
		files[i], files[j] = files[j], files[i]
	})

	for _, f := range files {
		if f.IsDir() {
			if indexMax <= 0 {
				return "", errors.New("存在太多嵌套目录，请清理")
			}
			newPath := filepath.Join(dirPath, f.Name())
			return randFile(newPath, indexMax-1)
		}
		if f.Name() == ".DS_Store" {
			continue
		}

		// 检查是否有扩展名（更可靠的方式）
		if ext := filepath.Ext(f.Name()); ext != "" {
			return filepath.Join(dirPath, f.Name()), nil
		}
	}

	return "", errors.New("未找到有效图片文件")
}

func drawImage(userinfo *userdata, score, add int) (data []byte, err error) {
	if userinfo.Picname == "" {
		err = errors.New("[ERROR]:签到图片获取失败")
		return
	}
	_, picName := filepath.Split(userinfo.Picname)
	picName, _, ok := strings.Cut(picName, "_")
	back, err := gg.LoadImage(userinfo.Picname)
	if err != nil {
		fileData, err := os.ReadFile(userinfo.Picname)
		if err != nil {
			return nil, fmt.Errorf("[ERROR]:读取签到图片失败: %v", err)
		}
		back, err = webp.Decode(bytes.NewReader(fileData))
		if err != nil {
			return nil, fmt.Errorf("[ERROR]:解码签到图片失败: %v", err)
		}
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
	// 统一字体解析和测量
	var names []string
	fontSize := 150.0
	data, err = file.GetLazyData(text.BoldFontFile, control.Md5File, true)
	if err != nil {
		return
	}
	setAndMeasure := func(fontSize float64) (nameW, nameH float64) {
		if err := canvas.ParseFontFace(data, fontSize); err != nil {
			return 0, 0
		}
		return canvas.MeasureString(userinfo.UserName)
	}
	nameW, nameH := setAndMeasure(fontSize)
	// 昵称范围
	textH := 300.0
	textW := float64(backDX) * 2 / 3
	// 如果文字超过长度了，比列缩小字体
	for {
		// 宽度适配
		if nameW > textW {
			fontSize *= textW / nameW
			nameW, nameH = setAndMeasure(fontSize)
			continue
		}

		// 分段计算
		names = splitIntoLines(userinfo.UserName, canvas, textW*0.75)
		totalHeight := nameH * 1.3 * float64(len(names))

		// 高度适配
		if totalHeight > textH && fontSize > 1 {
			fontSize *= textH / totalHeight
			nameW, nameH = setAndMeasure(fontSize)
			continue
		}

		break
	}
	// 计算垂直居中位置
	totalHeight := nameH * float64(len(names))
	startY := (textH-totalHeight)/2 + nameH/2

	// 绘制文本
	for i, line := range names {
		y := startY + float64(i)*nameH*1.3
		canvas.DrawStringAnchored(line, float64(backDX)/2, y, 0.5, 0.5)
	}

	// level
	if err = canvas.ParseFontFace(data, 72); err != nil {
		return
	}
	rankIndex, level, nextLevelScore := getLevel(userinfo.Level)
	nowLevel := rankIndex*5 + level
	rank := levelrank[rankIndex]
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
	canvas.DrawStringAnchored(levelrank[rankIndex], levelX+textW*1.2/2, 50+50, 0.5, 0.5)
	canvas.DrawStringAnchored(fmt.Sprintf("LV%d", nowLevel), levelX+textW*1.2/2, 50+100+50, 0.5, 0.5)

	if add == 0 {
		canvas.DrawStringAnchored(fmt.Sprintf("已连签 %d 天    %s: %d", userinfo.Continuous, wallet.GetWalletName(), score), float64(backDX)/2+100, 370-textH/2, 0.5, 0.5)
	} else {
		canvas.DrawStringAnchored(fmt.Sprintf("连签 %d 天 %s(+%d): %d", userinfo.Continuous, wallet.GetWalletName(), add+rankIndex*5, score), float64(backDX)/2+100, 370-textH/2, 0.5, 0.5)
	}
	// 绘制等级进度条
	if err = canvas.ParseFontFace(data, 50); err != nil {
		return
	}
	_, textH = canvas.MeasureString("/")
	switch {
	case nowLevel < 101 && add == 0:
		canvas.DrawStringAnchored(fmt.Sprintf("%d/%d", userinfo.Level, nextLevelScore), float64(backDX)/2, 455-textH, 0.5, 0.5)
	case nowLevel < 101:
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
	if ok {
		canvas.DrawStringAnchored("PID:"+picName, float64(backDX)/2, float64(backDY)-10-textH, 0.5, 0.5)
	}
	// 生成图片
	return imgfactory.ToBytes(canvas.Image())
}
func getLevel(count int) (int, int, int) {
	rankMax := len(levelrank) - 1
	i := 10
	for k := 0; k < rankMax; k++ {
		for j := 0; j < 5; j++ {
			if count < i {
				return k, j, i
			}
			i += (k + 1) * 30
		}
	}
	return rankMax, 1, i
}

// 将字符串分割成多行，确保每行不超过最大宽度
func splitIntoLines(s string, canvas *gg.Context, maxWidth float64) []string {
	var lines []string
	runes := []rune(s)

	for len(runes) > 0 {
		var currentLine []rune
		currentWidth := 0.0

		for i, r := range runes {
			w, _ := canvas.MeasureString(string(r))
			if currentWidth+w > maxWidth {
				if i == 0 { // 单个字符超长
					currentLine = runes[:1]
					runes = runes[1:]
				} else {
					currentLine = runes[:i]
					runes = runes[i:]
				}
				break
			}
			currentWidth += w
		}

		if len(currentLine) == 0 { // 处理剩余字符
			currentLine = runes
			runes = nil
		}

		lines = append(lines, string(currentLine))
	}

	return lines
}
