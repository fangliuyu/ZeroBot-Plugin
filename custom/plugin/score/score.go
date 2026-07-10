// Package score 签到系统
package score

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	fcext "github.com/FloatTech/floatbox/ctxext"
	"github.com/FloatTech/floatbox/process"
	"github.com/FloatTech/rendercard"
	ctrl "github.com/FloatTech/zbpctrl"
	control "github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"

	// 数据库
	"github.com/FloatTech/AnimeAPI/wallet"
	setu "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/setu" // 抽老婆
	sql "github.com/FloatTech/sqlite"

	// 图片输出
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/web"
	"github.com/FloatTech/gg"
	"github.com/FloatTech/gg/factory"
	"github.com/FloatTech/gg/fio"
	"github.com/FloatTech/zbputils/img/text"

	"golang.org/x/image/webp"
)

type score struct {
	db sql.Sqlite
	sync.RWMutex
}

// 用户数据信息
type userdata struct {
	UID        int64  // `Userid`
	UserName   string // `User`
	UpdatedAt  int64  // `签到时间`
	Continuous int    // `连续签到次数`
	Level      int    // `决斗者等级`
	Picname    string // `签到图片`
}

const cost = 100 // 获取签到背景所需点数

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
	cacheOtherPath = engine.DataFolder() + "cache/"
	dbpath         = engine.DataFolder() + "score.db"
	scoredata      = &score{db: sql.New(dbpath)}
	getdb          = fcext.DoOnceOnSuccess(func(ctx *zero.Ctx) bool {
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
)

func init() {
	go func() {
		err := os.MkdirAll(cacheOtherPath, 0755)
		if err != nil {
			panic(err)
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return
		}
		cacheOtherPath = filepath.Join(homeDir, "Pictures")
		if err := os.MkdirAll(cacheOtherPath, 0755); err != nil {
			return
		}
	}()
	engine.OnFullMatchGroup([]string{"签到", "打卡"}, getdb, setu.InitImgPool).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		uid := ctx.Event.UserID
		userinfo := scoredata.getData(uid)
		userinfo.UID = uid
		userinfo.UserName = ctx.CardOrNickName(uid) // 更新昵称
		lasttime := time.Unix(userinfo.UpdatedAt, 0)
		score := wallet.GetWalletOf(uid)

		go setu.AddAPItoPool("游戏王")
		// 判断是否已经签到过了
		if time.Now().Format("2006/01/02") == lasttime.Format("2006/01/02") && uid != 2504407110 {
			rankIndex, level, currentScore, nextLevelScore := getLevel(userinfo.Level)
			nowLevel := rankIndex*5 + level
			rank := levelrank[rankIndex]
			_, picName := filepath.Split(userinfo.Picname)
			picName, _, ok := strings.Cut(picName, "_")
			text := ""
			if ok {
				text = "\n签到背景PID:" + picName
			}
			ctx.SendChain(message.Text("今天已经签到过了,已连签: ", userinfo.Continuous, "天",
				"\n阶级: ", rank,
				"\n等级: ", nowLevel, "(", userinfo.Level-currentScore, "/", nextLevelScore-currentScore, ")",
				"\n当前总", wallet.GetWalletName(), ": ", score,
				text,
				"\n\n花费", cost, wallet.GetWalletName(), "发送“获取签到背景”查看高清签到背景",
			))
			return
		}
		var wg sync.WaitGroup
		var syncerr error
		wg.Add(1)
		go func() {
			defer wg.Done()
			process.SleepAbout1sTo2s()
			picFile, err := initPic(0)
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
				userinfo.Continuous++
				add = int(math.Min(5, float64(userinfo.Continuous)))
			}
			userinfo.UpdatedAt = time.Now().Unix()
			rankIndex, level, _, _ := getLevel(userinfo.Level)
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
		back, err := fio.LoadImage(userinfo.Picname)
		if err != nil {
			fileData, err := os.ReadFile(userinfo.Picname)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:读取签到图片失败: ", err))
				return
			}
			back, err = webp.Decode(bytes.NewReader(fileData))
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:读取签到图片失败: ", err))
				return
			}
		}
		imgDX := back.Bounds().Dx()
		imgDY := back.Bounds().Dy()
		var data []byte
		if imgDX > imgDY {
			// 如果是横图
			data, err = drawImage(&userinfo, score, add, back)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
		} else {
			// 如果是横图
			data, err = drawYHImage(&userinfo, score, add, back)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
		}
		ctx.SendChain(message.ImageBytes(data))
	})
	engine.OnPrefixGroup([]string{"获取签到背景", "获得签到背景", "获取签到图片", "获得签到图片", "获取打卡背景", "获得打卡背景", "获取打卡图片", "获得打卡图片"}).Limit(ctxext.LimitByGroup).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			score := wallet.GetWalletOf(ctx.Event.UserID)
			if score < cost {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("你的", wallet.GetWalletName(), "不足", cost, ",无法获取签到背景。"))
				return
			}
			param := strings.TrimSpace(ctx.State["args"].(string))
			var uid int64
			switch {
			case len(ctx.Event.Message) > 1 && ctx.Event.Message[1].Type == "at":
				uid, _ = strconv.ParseInt(ctx.Event.Message[1].Data["qq"], 10, 64)
			case param == "":
				uid = ctx.Event.UserID
			default:
				paramUID, err := strconv.ParseInt(param, 10, 64)
				if err != nil {
					ctx.SendChain(message.Text("请输入正确的QQ号,", err))
					return
				}
				uid = paramUID
			}
			userinfo := scoredata.getData(uid)
			picFile := userinfo.Picname
			if picFile == "" || file.IsNotExist(picFile) {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.At(uid), message.Text("请先签到！"))
				return
			}
			// ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + picFile))
			pic, err := os.ReadFile(picFile)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]", err))
				return
			}
			if msgID := ctx.SendChain(message.ImageBytes(pic)).ID(); msgID != 0 {
				err := wallet.InsertWalletOf(ctx.Event.UserID, -cost)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("已扣除", cost, wallet.GetWalletName()))
			} else {
				_ = ctx.SetMessageEmojiLike(ctx.Event.MessageID, 5)
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
		userinfo.UID = uid
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

func initPic(idex int) (picFile string, err error) {
	if idex > 3 || rand.Intn(10) < 3 {
		if idex > 3 {
			logrus.Warningln("[score] lolicon下载图片失败,将从moehu下载图片:", err)
		}
		picFile, err = moehu(0)
		return picFile, err
	}
	picFile = setu.ImgPool.Pop("游戏王")
	logrus.Infoln("[score] 从ImgPool获取签到图片:", picFile)
	if picFile == "" || !file.IsExist(picFile) {
		return initPic(idex + 1)
	}
	return
}

// 下载图片
func moehu(idex int) (picFile string, err error) {
	if idex > 3 {
		logrus.Warningln("[score] moehu下载图片失败,将从alcy下载图片:", err)
		return otherPic(0)
	}
	newurl, err := web.HeadRequestURL("https://img.moehu.org/pic.php?id=yu-gi-oh")
	if err != nil {
		logrus.Warningln(err)
		return moehu(idex + 1)
	}
	logrus.Warningln("[score] 新链接:", newurl)
	_, fileName := filepath.Split(newurl)
	masterDir := filepath.Join(cacheOtherPath, fileName)
	err = os.MkdirAll(masterDir, 0755)
	if err != nil {
		logrus.Warningln(err)
		return otherPic(idex + 1)
	}
	picFile = filepath.Join(masterDir, fileName)
	err = setu.DownloadPicTo(newurl, picFile)
	if err != nil {
		logrus.Warningln(err)
		return moehu(idex + 1)
	}
	setu.ImgPool.AddLocal("游戏王", 0, picFile)
	return
}

// 下载图片
func otherPic(idex int) (picFile string, err error) {
	if idex > 3 {
		logrus.Warningln("[score] alcy下载图片失败,将从本地抽选:", err)
		return randFile(setu.CacheDir, 3)
	}
	resp, err := web.HeadRequestURL("https://t.alcy.cc/ycy/")
	if err != nil {
		logrus.Warningln(err)
		return otherPic(idex + 1)
	}
	logrus.Warningln("[score] 新链接:", resp)
	_, fileName := filepath.Split(resp)
	picFile = filepath.Join(cacheOtherPath, fileName)
	err = setu.DownloadPicTo(resp, picFile)
	if err != nil {
		logrus.Warningln(err)
		return otherPic(idex + 1)
	}
	setu.ImgPool.AddLocal("二次元", 0, picFile)
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

		if ext := filepath.Ext(f.Name()); ext != "" {
			filePath := filepath.Join(fullPath, f.Name())
			setu.ImgPool.AddLocal("游戏王", 0, filePath)
			return filePath, nil
		}
	}

	return "", errors.New("未找到有效图片文件")
}

func drawImage(userinfo *userdata, score, add int, back image.Image) (img_byte []byte, err error) {
	if userinfo.Picname == "" {
		err = errors.New("[ERROR]:签到图片获取失败")
		return
	}
	_, fileName := filepath.Split(userinfo.Picname)
	picName, _, ok := strings.Cut(fileName, "_")
	imgDX := back.Bounds().Dx()
	imgDY := back.Bounds().Dy()
	backDX := 1920

	imgDW := backDX - 100
	scale := float64(imgDW) / float64(imgDX)
	imgDH := int(float64(imgDY) * scale)
	back = factory.Size(back, imgDW, imgDH).Image()

	backDY := imgDH + 500 + 10 + 50 + 10
	canvas := gg.NewContext(backDX, backDY)
	// 放置毛玻璃背景
	backBlurW := float64(imgDW) * (float64(backDY) / float64(imgDH))
	canvas.DrawImageAnchored(imaging.Blur(factory.Size(back, int(backBlurW), backDY).Image(), 8), backDX/2, backDY/2, 0.5, 0.5)
	canvas.DrawRectangle(1, 1, float64(backDX), float64(backDY))
	canvas.SetLineWidth(3)
	canvas.SetRGBA255(255, 255, 255, 100)
	canvas.StrokePreserve()
	canvas.SetRGBA255(255, 255, 255, 140)
	canvas.Fill()
	// 信息框
	rectangleW := float64(backDX) - 20 - 20
	canvas.DrawRoundedRectangle(20, 20, rectangleW, 450-20, (450-20)/5)
	canvas.SetLineWidth(6)
	canvas.SetDash(20.0, 10.0, 0)
	canvas.SetRGBA255(255, 255, 255, 255)
	canvas.Stroke()
	// 放置头像
	getAvatar, err := web.GetData("http://q4.qlogo.cn/g?b=qq&nk=" + strconv.FormatInt(userinfo.UID, 10) + "&s=640")
	if err != nil {
		return
	}
	avatar, _, err := image.Decode(bytes.NewReader(getAvatar))
	if err != nil {
		return
	}
	avatarf := factory.Size(avatar, 270, 270)
	canvas.DrawCircle(50+float64(avatarf.W())/2, 50+float64(avatarf.H())/2, float64(avatarf.W())/2+2)
	canvas.SetLineWidth(3)
	canvas.SetDash()
	canvas.SetRGBA255(255, 255, 255, 255)
	canvas.Stroke()
	canvas.DrawImage(avatarf.Circle(0).Image(), 50, 50)

	canvas.SetRGB(0, 0, 0)
	data, err := file.GetLazyData(text.BoldFontFile, control.Md5File, true)
	if err != nil {
		return
	}

	// level
	if err = canvas.ParseFontFace(data, 72); err != nil {
		return
	}
	rankIndex, level, currentScore, nextLevelScore := getLevel(userinfo.Level)
	nowLevel := rankIndex*5 + level
	rank := levelrank[rankIndex]
	textW, textH := canvas.MeasureString(rank)
	levelX := float64(backDX) - 20 - 20 - textW*1.2
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
		canvas.DrawStringAnchored(fmt.Sprintf("%d/%d", userinfo.Level-currentScore, nextLevelScore-currentScore), float64(backDX)/2, 455-textH, 0.5, 0.5)
	case nowLevel < 101:
		canvas.DrawStringAnchored(fmt.Sprintf("(%d+%d)/%d", userinfo.Level-currentScore-add, add, nextLevelScore-currentScore), float64(backDX)/2, 455-textH, 0.5, 0.5)
	default:
		canvas.DrawStringAnchored("Max/Max", float64(backDX)/2, 455-textH, 0.5, 0.5)
	}
	if ok {
		canvas.DrawStringAnchored("PID:"+picName, 200, float64(backDY)-10-textH, 0.5, 0.5)
	}
	canvas.DrawStringAnchored(
		"花费"+strconv.Itoa(cost)+wallet.GetWalletName()+"发送“获取签到背景”获取高清图片",
		float64(backDX)/2-300, float64(backDY)-10-textH, 0, 0.5,
	)

	// 放置昵称
	// 统一字体解析和测量
	var names []string
	fontSize := 150.0
	setAndMeasure := func(fontSize float64) (nameW, nameH float64) {
		if err := canvas.ParseFontFace(data, fontSize); err != nil {
			return 0, 0
		}
		return canvas.MeasureString(userinfo.UserName)
	}
	nameW, nameH := setAndMeasure(fontSize)
	// 昵称范围
	textH = 300.0
	textW = levelX - 20 - 300
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

	// 创建彩虹条
	grad := gg.NewLinearGradient(0, 450-3.5, float64(backDX), 450+3.5)
	grad.AddColorStop(0, color.RGBA{G: 255, A: 255})
	grad.AddColorStop(0.35, color.RGBA{B: 255, A: 255})
	grad.AddColorStop(0.5, color.RGBA{R: 255, A: 255})
	grad.AddColorStop(0.65, color.RGBA{B: 255, A: 255})
	grad.AddColorStop(1, color.RGBA{G: 255, A: 255})
	canvas.SetStrokeStyle(grad)
	canvas.SetLineWidth(7)
	// 设置长度
	gradMax := rectangleW - 2*(450-20)/5
	LevelLength := gradMax * (float64(userinfo.Level-currentScore) / float64(nextLevelScore-currentScore))
	canvas.MoveTo((float64(backDX)-LevelLength)/2, 450)
	canvas.LineTo((float64(backDX)+LevelLength)/2, 450)
	canvas.ClosePath()
	canvas.Stroke()
	// 放置图片
	canvas.DrawImageAnchored(back, backDX/2, imgDH/2+475, 0.5, 0.5)
	// 生成图片
	return factory.ToBytes(canvas.Image())
}

func drawYHImage(userinfo *userdata, score, add int, back image.Image) (img_byts []byte, err error) {
	if userinfo.Picname == "" {
		err = errors.New("[ERROR]:签到图片获取失败")
		return
	}
	fontdata, err := file.GetLazyData(text.GlowSansFontFile, control.Md5File, false)
	if err != nil {
		return
	}

	rankIndex, level, currentLevelScore, nextLevelScore := getLevel(userinfo.Level)
	currentScore := userinfo.Level - currentLevelScore
	nextScore := nextLevelScore - currentLevelScore

	canvasWidth := 1920
	canvasHeight := 1080
	canvas := gg.NewContext(canvasWidth, canvasHeight)
	cw, ch := float64(canvas.W()), float64(canvas.H())
	// 计算卡片主体区域高度（画布高度的60%）
	scw, sch := cw*6/10, ch*6/10
	// 计算头像的宽高
	aw, ah := (ch-sch)/2/2/2*3, (ch-sch)/2/2/2*3

	colors := gg.TakeThemeColorsKMeans(back, 3)
	canvas.SetColor(colors[0])
	canvas.Clear()

	back = factory.Limit(back, canvasWidth*6/10, canvasHeight*8/10)

	var blurback, backshadowimg, avatarimg, avatarbackimg, avatarshadowimg, whitetext, blacktext, linearGradient image.Image
	wg := &sync.WaitGroup{}
	wg.Add(8)

	go func() {
		defer wg.Done()
		backImg := applyHorizontalFade(scaleImageToWidth(back, canvasWidth), 0.7)
		blurback = rendercard.Fillet(backImg, 12)
	}()

	go func() {
		defer wg.Done()
		// 生成背景阴影效果
		pureblack := gg.NewContext(back.Bounds().Dx()*101/100, back.Bounds().Dy()*101/100)
		pureblack.SetRGBA255(0, 0, 0, 255)
		pureblack.Clear()
		backshadowimg = pureblack.Image()
	}()

	go func() {
		defer wg.Done()
		// 生成等级进度条
		linearGradient = drawLinearGradient(cw-scw, canvas.FontHeight()/2, float64(currentScore)/float64(nextScore))
	}()

	go func() {
		defer wg.Done()
		// 处理用户头像
		getAvatar, err := web.GetData("http://q4.qlogo.cn/g?b=qq&nk=" + strconv.FormatInt(userinfo.UID, 10) + "&s=640")
		if err != nil {
			return
		}
		avatar, _, err := image.Decode(bytes.NewReader(getAvatar))
		if err != nil {
			return
		}

		// 计算头像缩放比例
		isc := (ch - sch) / 2 / 2 / 2 * 3 / float64(avatar.Bounds().Dy())

		scavatar := gg.NewContext(int(aw), int(ah))

		// 缩放并居中头像
		scavatar.ScaleAbout(isc, isc, aw/2, ah/2)
		scavatar.DrawImageAnchored(avatar, scavatar.W()/2, scavatar.H()/2, 0.5, 0.5)
		scavatar.Identity()

		avatarimg = rendercard.Fillet(scavatar.Image(), 8) // 圆角处理
	}()

	go func() {
		defer wg.Done()
		// 生成名字阴影
		avatarshadowimg = imaging.Blur(customrectangle(cw, ch, aw, ah, color.Black), 8)
	}()

	go func() {
		defer wg.Done()
		// 生成名字模糊背景
		avatarbackimg = customrectangle(cw, ch, aw, ah, colors[0])
	}()

	go func() {
		defer wg.Done()
		whitetext, err = customtext(userinfo, score, add, rankIndex, level, fontdata, cw, ch, aw, ah, color.White)
		if err != nil {
			return
		}
	}()

	go func() {
		defer wg.Done()
		blacktext, err = customtext(userinfo, score, add, rankIndex, level, fontdata, cw, ch, aw, ah, color.Black)
		if err != nil {
			return
		}
	}()

	wg.Wait()
	if backshadowimg == nil || avatarimg == nil || avatarbackimg == nil || avatarshadowimg == nil || whitetext == nil || blacktext == nil {
		err = errors.New("图片渲染失败")
		return
	}

	// 1. 绘制模糊背景
	canvas.DrawImageAnchored(
		blurback,
		canvas.W()/2-blurback.Bounds().Dx()*25/100,
		-blurback.Bounds().Dy()/10,
		0.5, 0,
	)

	// 2. 绘制签到图阴影
	canvas.DrawImageAnchored(backshadowimg, canvas.W()*98/100-backshadowimg.Bounds().Dx()/2, canvas.H()/2+canvas.H()/100, 0.5, 0.5)
	canvas.DrawImageAnchored(back, canvas.W()*98/100-backshadowimg.Bounds().Dx()/2, canvas.H()/2+canvas.H()/100, 0.5, 0.5)

	// 4. 绘制头像相关元素
	canvas.DrawImage(avatarshadowimg, 0, 0)
	canvas.DrawImage(avatarbackimg, 0, 0)
	canvas.DrawImageAnchored(avatarimg, int((ch-sch)/2/2), int((ch-sch)/2/2), 0.5, 0.5)

	// 5. 绘制等级进度条
	canvas.DrawImageAnchored(linearGradient, int((ch-sch)/2/2+aw/2+aw/40*2+cw*0.01), int((ch-sch)/2/2), 0, 0.5)

	// 6. 绘制文字（黑色阴影 + 白色文字，产生立体效果）
	canvas.DrawImage(blacktext, 3, 3) // 偏移3像素的黑色文字
	canvas.DrawImage(whitetext, 0, 0) // 白色文字

	// 生成图片
	return factory.ToBytes(canvas.Image())
}

// 将图片缩放到指定宽度（保持比例）
func scaleImageToWidth(img image.Image, targetWidth int) image.Image {
	bounds := img.Bounds()
	originalWidth := float64(bounds.Dx())
	originalHeight := float64(bounds.Dy())

	scale := float64(targetWidth) / originalWidth
	targetHeight := int(originalHeight * scale)

	dc := gg.NewContext(targetWidth, targetHeight)
	dc.DrawImageAnchored(img, 0, 0, 0, 0)
	dc.Scale(scale, scale)
	dc.DrawImageAnchored(img, 0, 0, 0, 0)

	return dc.Image()
}

// 应用水平渐变透明度（从指定比例位置开始变透明）
func applyHorizontalFade(img image.Image, fadeStartRatio float64) image.Image {
	bounds := img.Bounds()
	width := float64(bounds.Dx())
	height := float64(bounds.Dy())

	// 创建一个新画布，尺寸与原图相同
	resultDC := gg.NewContext(int(width), int(height))

	// 创建渐变蒙版
	maskDC := gg.NewContext(int(width), int(height))
	gradient := gg.NewLinearGradient(0, 0, width, 0)                // 注意：Y坐标保持一致
	gradient.AddColorStop(0, color.RGBA{0, 0, 0, 127})              // 起始：完全不透明
	gradient.AddColorStop(fadeStartRatio, color.RGBA{0, 0, 0, 127}) // 起始：完全不透明
	gradient.AddColorStop(0.95, color.RGBA{0, 0, 0, 0})             // 提前结束过滤图框：完全透明
	gradient.AddColorStop(1, color.RGBA{0, 0, 0, 0})                // 结束：完全透明

	maskDC.SetFillStyle(gradient)
	maskDC.DrawRectangle(0, 0, width, height)
	maskDC.Fill()

	// 先设置蒙版
	resultDC.SetMask(maskDC.AsMask())

	// 原图绘制
	resultDC.DrawImage(img, 0, 0)

	return resultDC.Image()
}

// customrectangle 绘制自定义圆角矩形背景
// 参数说明：
//
//	aw, ah: 头像区域宽高
//	namew: 用户名宽度
//	rtgcolor: 矩形颜色
func customrectangle(cw, ch, aw, ah float64, rtgcolor color.Color) (img image.Image) {
	canvas := gg.NewContext(int(cw), int(ch))
	sch := ch * 6 / 10
	canvas.SetColor(rtgcolor)

	// 绘制头像背景圆角矩形
	canvas.DrawRoundedRectangle((ch-sch)/2/2-aw/2-aw/40, (ch-sch)/2/2-aw/2-ah/40, aw+aw/40*2, ah+ah/40*2, 8)
	canvas.Fill()

	img = canvas.Image()
	return
}

// customtext 生成签到卡片上的文字内容
// 参数说明：
//
//	a: 用户数据
//	fontdata: 字体数据
//	cw, ch: 画布宽高
//	aw: 头像宽度
//	ah: 头像高度
//	textcolor: 文字颜色
func customtext(a *userdata, score, add, rankIndex, level int, fontdata []byte, cw, ch, aw, ah float64, textcolor color.Color) (img image.Image, err error) {
	canvas := gg.NewContext(int(cw), int(ch))
	canvas.SetColor(textcolor)
	scw, sch := cw*6/10, ch*6/10
	err = canvas.ParseFontFace(fontdata, (ch-sch)/2/2/2)
	if err != nil {
		return
	}
	canvas.DrawStringAnchored(a.UserName, (ch-sch)/2/2+aw/2+aw/40*2+cw*0.01, (ch-sch)/2/2-ah/2/2, 0, 0.5)

	nowLevel := rankIndex*5 + level
	canvas.DrawStringAnchored(levelrank[rankIndex]+" (Level "+strconv.Itoa(nowLevel)+")", (ch-sch)/2/2+aw/2+aw/40*2+cw*0.01, (ch-sch)/2/2+ah/2/2, 0, 0.5)

	err = canvas.ParseFontFace(fontdata, (ch-sch)/2/2/3*2)
	if err != nil {
		return
	}
	_, fileName := filepath.Split(a.Picname)
	picName, _, ok := strings.Cut(fileName, "_")
	if ok {
		pidw, _ := canvas.MeasureString(picName)
		canvas.DrawStringAnchored("PID: "+picName, cw*0.93-pidw/2, (ch-sch)/2-ah, 0.5, 0.5)
	}

	err = canvas.ParseFontFace(fontdata, (ch-sch)/2/5*3)
	if err != nil {
		return
	}
	tempfh := canvas.FontHeight()
	canvas.DrawStringAnchored(strconv.Itoa(score)+wallet.GetWalletName(), ((cw-scw)-(cw/3-scw/2))/16, ch-tempfh/3-tempfh/2, 0, 0.5)

	err = canvas.ParseFontFace(fontdata, (ch-sch)/2/4)
	if err != nil {
		return
	}
	canvas.DrawStringAnchored(
		"连签"+strconv.Itoa(a.Continuous)+"天  +"+strconv.Itoa(add+rankIndex*5)+wallet.GetWalletName(),
		((cw-scw)-(cw/3-scw/2))/16, ch-tempfh/3-tempfh*1.5, 0, 0.5,
	)

	err = canvas.ParseFontFace(fontdata, (ch-sch)/2/2/3)
	if err != nil {
		return
	}
	txt := "花费" + strconv.Itoa(cost) + wallet.GetWalletName() + "发送“获取签到背景”获取高清图片"
	txtw, txth := canvas.MeasureString(txt)
	canvas.DrawStringAnchored(txt, cw*0.98-txtw/2, ch-txth, 0.5, 0)
	img = canvas.Image()
	return
}

func drawLinearGradient(w, h, sc float64) (img image.Image) {
	canvas := gg.NewContext(int(w), int(h))
	grad := gg.NewLinearGradient(0, h, w, 0)
	grad.AddColorStop(0, color.RGBA{G: 255, A: 255})
	grad.AddColorStop(sc-0.02, color.RGBA{G: 255, A: 255})
	grad.AddColorStop(sc, color.RGBA{B: 255, A: 255})
	grad.AddColorStop(sc+0.02, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	grad.AddColorStop(1, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	canvas.SetStrokeStyle(grad)
	canvas.SetLineWidth(h)
	canvas.MoveTo(0, h/2)
	canvas.LineTo(w, h/2)
	canvas.ClosePath()
	canvas.Stroke()
	img = canvas.Image()
	return
}

// 根据签到积分获取等级信息
// 返回值：等级段位索引、当前段位内等级、 当前所需的积分、下一段位所需积分
func getLevel(count int) (int, int, int, int) {
	rankMax := len(levelrank) - 1
	i := 10
	now_socore := 0
	for k := range rankMax {
		for j := range 5 {
			if count < i {
				return k, j, now_socore, i
			}
			now_socore = i
			i += (k + 1) * 30
		}
	}
	return rankMax, 1, now_socore, i
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
