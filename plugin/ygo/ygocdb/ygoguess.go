// Package ygo 一些关于ygo的插件
package ygo

import (
	"image/color"
	"math"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FloatTech/floatbox/file"
	zbmath "github.com/FloatTech/floatbox/math"
	"github.com/FloatTech/imgfactory"
	ctrl "github.com/FloatTech/zbpctrl"
	control "github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension/single"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/FloatTech/gg"
)

// GameInfo 游戏信息
type GameInfo struct {
	UID         int64
	CID         int
	Name        string
	Pic         []byte
	Info        []string
	LastTime    time.Time // 距离上次回答时间
	Worry       int       // 错误次数
	TickCount   int       // 提示次数
	AnswerCount int       // 问答次数
}

var (
	gameRoom sync.Map
	engine   = control.Register("ygoguess", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "游戏王猜卡游戏",
		Help:             "- 猜卡游戏\n- 我猜xxx",
	}).ApplySingle(single.New(
		single.WithKeyFn(func(ctx *zero.Ctx) int64 { return ctx.Event.GroupID }),
		single.WithPostFn[int64](func(ctx *zero.Ctx) {
			ctx.Break()
			ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("已经有正在进行的游戏...")))
		}),
	))
)

func init() {
	engine.OnRegex("^(黑边|反色|马赛克|旋转|切图)?猜卡游戏$", zero.OnlyGroup).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		gid := ctx.Event.GroupID
		info, ok := gameRoom.Load(gid)
		if ok {
			gameInfo := info.(GameInfo)
			if time.Since(gameInfo.LastTime).Seconds() < 105 {
				ctx.SendChain(message.Text("已经有正在进行的游戏:"))
				mid := ctx.SendChain(message.ImageBytes(gameInfo.Pic))
				if mid.ID() != 0 {
					ctx.SendChain(message.Text("请回答该图的卡名\n以“我猜xxx”格式回答\n(xxx需包含卡名1/4以上)\n或发“提示”得提示;“取消”结束游戏"))
				}
				return
			}
		}
		ctx.SendChain(message.Text("正在准备题目,请稍等"))
		data := drawCard()
		picFile := cachePath + strconv.Itoa(data.ID) + ".jpg"
		if file.IsNotExist(picFile) {
			url := picherf + strconv.Itoa(data.ID) + ".jpg"
			err := file.DownloadTo(url, picFile)
			if err != nil {
				ctx.SendChain(message.Text("图片下载失败,可能被风控", err))
				return
			}
		}
		// 对卡图做处理
		pictrue, err := randPicture(picFile, data.Text.Types)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]", err))
			return
		}
		gameInfo := GameInfo{
			UID:      ctx.Event.UserID,
			CID:      data.ID,
			Name:     data.CnName,
			LastTime: time.Now(),
			Pic:      pictrue,
			Info:     []string{getTips(data, 0), getTips(data, 1), getTips(data, 2)},
		}
		gameRoom.Store(gid, gameInfo)
		picPath := cachePath + strconv.Itoa(gameInfo.CID) + ".jpg"
		pic, err := os.ReadFile(picPath)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]", err))
			return
		}
		length := zbmath.Ceil(len([]rune(gameInfo.Name)), 4)
		mid := ctx.SendChain(message.ImageBytes(gameInfo.Pic))
		if mid.ID() != 0 {
			ctx.SendChain(message.Text("请回答该图的卡名\n以“我猜xxx”格式回答\n(xxx需包含卡名1/4以上)\n或发“提示”得提示;“取消”结束游戏"))
		}
		// 进行猜卡环节
		recv, cancel := zero.NewFutureEvent("message", 1, true, zero.RegexRule("^((我猜.+)|提示|取消)$"), zero.OnlyGroup, zero.CheckGroup(ctx.Event.GroupID)).Repeat()
		defer cancel()
		subtime := time.Since(gameInfo.LastTime)
		if subtime.Seconds() > 105 {
			subtime = 0
		}
		tick := time.NewTimer(105*time.Second - subtime)
		over := time.NewTimer(120*time.Second - subtime)
		for {
			select {
			case <-tick.C:
				tick.Stop()
				ctx.SendChain(message.Text("还有15s作答时间"))
			case <-over.C:
				tick.Stop()
				over.Stop()
				msgID := ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID,
					message.Text("时间超时,游戏结束\n卡名是:\n", gameInfo.Name, "\n"),
					message.ImageBytes(pic)))
				if msgID.ID() == 0 {
					ctx.SendChain(message.Text("时间超时,游戏结束\n图片发送失败,可能被风控\n答案是:", gameInfo.Name))
				}
				defer gameRoom.Delete(gid)
				return
			case c := <-recv:
				time.Sleep(time.Millisecond * time.Duration(10+rand.Intn(50)))
				msgID := c.Event.MessageID
				answer := c.Event.Message.String()
				_, after, ok := strings.Cut(answer, "我猜")
				if ok {
					if len([]rune(after)) < length {
						ctx.Send(message.ReplyWithMessage(msgID, message.Text("请输入", length, "字以上")))
						continue
					}
					answer = after
				}
				switch {
				case answer == "取消":
					if c.Event.UserID != ctx.Event.UserID {
						ctx.Send(message.ReplyWithMessage(msgID, message.Text("你无权限取消")))
						continue
					}
					tick.Stop()
					over.Stop()
					defer gameRoom.Delete(gid)
					gameInfo.Worry = zbmath.Max(gameInfo.Worry, 6)
					msgID := ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID,
						message.Text("游戏已取消\n卡名是:\n", gameInfo.Name, "\n"),
						message.ImageBytes(pic)))
					if msgID.ID() == 0 {
						ctx.SendChain(message.Text("游戏已取消\n图片发送失败,可能被风控\n答案是:", gameInfo.Name))
					}
					return
				case answer == "提示" && gameInfo.TickCount > 2:
					tick.Reset(105 * time.Second)
					over.Reset(120 * time.Second)
					ctx.Send(message.ReplyWithMessage(msgID, message.Text("已经没有提示了哦,加油啊")))
					continue
				case answer == "提示":
					// gameInfo.Worry++
					tick.Reset(105 * time.Second)
					over.Reset(120 * time.Second)
					ctx.Send(message.ReplyWithMessage(msgID, message.Text(gameInfo.Info[gameInfo.TickCount])))
					gameInfo.TickCount++
					continue
				case strings.Contains(gameInfo.Name, answer):
					tick.Stop()
					over.Stop()
					defer gameRoom.Delete(gid)
					msgID := ctx.Send(message.ReplyWithMessage(msgID,
						message.Text("太棒了,你猜对了!\n卡名是:\n", gameInfo.Name, "\n"),
						message.ImageBytes(pic)))
					if msgID.ID() == 0 {
						ctx.SendChain(message.Text("太棒了,你猜对了!\n图片发送失败,可能被风控\n答案是:", gameInfo.Name))
					}
					return
				case gameInfo.AnswerCount >= 5:
					tick.Stop()
					over.Stop()
					defer gameRoom.Delete(gid)
					msgID := ctx.Send(message.ReplyWithMessage(msgID,
						message.Text("次数到了,很遗憾没能猜出来\n卡名是:\n", gameInfo.Name, "\n"),
						message.ImageBytes(pic)))
					if msgID.ID() == 0 {
						ctx.SendChain(message.Text("次数到了,很遗憾没能猜出来\n图片发送失败,可能被风控\n答案是:", gameInfo.Name))
					}
					return
				default:
					gameInfo.Worry++
					gameInfo.AnswerCount++
					tick.Reset(105 * time.Second)
					over.Reset(120 * time.Second)
					ctx.Send(message.ReplyWithMessage(msgID, message.Text("答案不对哦,还有"+strconv.Itoa(6-gameInfo.AnswerCount)+"次回答机会,加油啊~")))
				}
			}
		}
	})
}

// 随机选择
func randPicture(picFile, cardType string) ([]byte, error) {
	types := []func(*imgfactory.Factory) ([]byte, error){
		backPic, mosaic, doublePicture, cutPic, randSet,
	}
	pic, err := gg.LoadImage(picFile)
	if err != nil {
		return nil, err
	}
	dst := imgfactory.Size(pic, pic.Bounds().Dx(), pic.Bounds().Dy())
	if strings.Contains(cardType, "灵摆") {
		dst = dst.Clip(370-29, 358-105, 29, 105)
	} else {
		dst = dst.Clip(351-51, 408-108, 51, 108)
	}
	dst = imgfactory.Size(dst.Image(), 256*5, 256*5)
	id := rand.Intn(len(types))
	println("\n*********猜卡ID:", id, " *********\n")
	dstfunc := types[id]
	picbytes, err := dstfunc(dst)
	return picbytes, err
}

// 获取黑边
func backPic(dst *imgfactory.Factory) ([]byte, error) {
	bounds := dst.Image().Bounds()
	returnpic := imgfactory.NewFactoryBG(dst.W(), dst.H(), color.NRGBA{255, 255, 255, 255}).Image()

	for y := bounds.Min.Y; y <= bounds.Max.Y; y++ {
		for x := bounds.Min.X; x <= bounds.Max.X; x++ {
			a := dst.Image().At(x, y)
			colorA := color.NRGBAModel.Convert(a).(color.NRGBA)
			b := dst.Image().At(x+1, y)
			colorB := color.NRGBAModel.Convert(b).(color.NRGBA)
			c := dst.Image().At(x, y+1)
			colorC := color.NRGBAModel.Convert(c).(color.NRGBA)
			if math.Sqrt(float64((colorA.R-colorB.R)*(colorA.R-colorB.R)+(colorA.G-colorB.G)*(colorA.G-colorB.G)+(colorA.B-colorB.B)*(colorA.B-colorB.B))) > 15 {
				returnpic.Set(x, y, color.NRGBA{0, 0, 0, 0})
			} else if math.Sqrt(float64((colorA.R-colorC.R)*(colorA.R-colorC.R)+(colorA.G-colorC.G)*(colorA.G-colorC.G)+(colorA.B-colorC.B)*(colorA.B-colorC.B))) > 15 {
				returnpic.Set(x, y, color.NRGBA{0, 0, 0, 0})
			}
		}
	}
	return imgfactory.ToBytes(returnpic)
}

// 旋转
func doublePicture(dst *imgfactory.Factory) ([]byte, error) {
	b := dst.Image().Bounds()
	pic := dst.FlipH().FlipV()
	for y := b.Min.Y; y <= b.Max.Y; y++ {
		for x := b.Min.X; x <= b.Max.X; x++ {
			a := pic.Image().At(x, y)
			c := color.NRGBAModel.Convert(a).(color.NRGBA)
			a1 := dst.Image().At(x, y)
			c1 := color.NRGBAModel.Convert(a1).(color.NRGBA)
			switch {
			case y > x && x < b.Max.X/2 && y < b.Max.Y/2:
				dst.Image().Set(x, y, c)
			case y < x && x > b.Max.X/2 && y > b.Max.Y/2:
				dst.Image().Set(x, y, c)
			case y > b.Max.Y-x && x < b.Max.X/2 && y > b.Max.Y/2:
				dst.Image().Set(x, y, c)
			case y < b.Max.Y-x && x > b.Max.X/2 && y < b.Max.Y/2:
				dst.Image().Set(x, y, c)
			default:
				dice := rand.Intn(10)
				if dice < 3 {
					dst.Image().Set(x, y, color.NRGBA{
						R: c1.R,
						G: c1.G,
						B: c1.B,
						A: 255,
					})
				} else {
					dst.Image().Set(x, y, color.NRGBA{
						R: 255,
						G: 255,
						B: 255,
						A: 255,
					})
				}
			}
		}
	}
	return imgfactory.ToBytes(dst.Image())
}

// 马赛克
func mosaic(dst *imgfactory.Factory) ([]byte, error) {
	b := dst.Image().Bounds()
	markSize := (b.Max.X * (5 - rand.Intn(4))) / 100

	for yOfMarknum := 0; yOfMarknum <= zbmath.Ceil(b.Max.Y, markSize); yOfMarknum++ {
		for xOfMarknum := 0; xOfMarknum <= zbmath.Ceil(b.Max.X, markSize); xOfMarknum++ {
			a := dst.Image().At(xOfMarknum*markSize+markSize/2, yOfMarknum*markSize+markSize/2)
			cc := color.NRGBAModel.Convert(a).(color.NRGBA)
			for y := 0; y < markSize; y++ {
				for x := 0; x < markSize; x++ {
					xOfPic := xOfMarknum*markSize + x
					yOfPic := yOfMarknum*markSize + y
					dst.Image().Set(xOfPic, yOfPic, cc)
				}
			}
		}
	}
	return imgfactory.ToBytes(dst.Blur(3).Image())
}

// 随机切割
func cutPic(dst *imgfactory.Factory) ([]byte, error) {
	indexOfx := rand.Intn(3)
	indexOfy := rand.Intn(3)
	indexOfx2 := rand.Intn(3)
	indexOfy2 := rand.Intn(3)
	b := dst.Image()
	bx := b.Bounds().Max.X / 3
	by := b.Bounds().Max.Y / 3
	returnpic := imgfactory.NewFactoryBG(dst.W(), dst.H(), color.NRGBA{255, 255, 255, 255})

	for yOfMarknum := b.Bounds().Min.Y; yOfMarknum <= b.Bounds().Max.Y; yOfMarknum++ {
		for xOfMarknum := b.Bounds().Min.X; xOfMarknum <= b.Bounds().Max.X; xOfMarknum++ {
			if xOfMarknum == bx || yOfMarknum == by || xOfMarknum == bx*2 || yOfMarknum == by*2 {
				// 黑框
				returnpic.Image().Set(xOfMarknum, yOfMarknum, color.NRGBA{0, 0, 0, 0})
			}
			if xOfMarknum >= bx*indexOfx && xOfMarknum < bx*(indexOfx+1) {
				if yOfMarknum >= by*indexOfy && yOfMarknum < by*(indexOfy+1) {
					a := dst.Image().At(xOfMarknum, yOfMarknum)
					cc := color.NRGBAModel.Convert(a).(color.NRGBA)
					returnpic.Image().Set(xOfMarknum, yOfMarknum, cc)
				}
			}
			if xOfMarknum >= bx*indexOfx2 && xOfMarknum < bx*(indexOfx2+1) {
				if yOfMarknum >= by*indexOfy2 && yOfMarknum < by*(indexOfy2+1) {
					a := dst.Image().At(xOfMarknum, yOfMarknum)
					cc := color.NRGBAModel.Convert(a).(color.NRGBA)
					returnpic.Image().Set(xOfMarknum, yOfMarknum, cc)
				}
			}
		}
	}
	return imgfactory.ToBytes(returnpic.Image())
}

// 乱序
func randSet(dst *imgfactory.Factory) ([]byte, error) {
	b := dst.Image().Bounds()
	w, h := b.Max.X/3, b.Max.Y/3
	returnpic := imgfactory.NewFactoryBG(dst.W(), dst.H(), color.NRGBA{255, 255, 255, 255})

	mapPicOfX := []int{0, 0, 0, 1, 1, 1, 2, 2, 2}
	mapPicOfY := []int{0, 1, 2, 0, 1, 2, 0, 1, 2}

	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			index := 0
			mapfaceX := mapPicOfX[index]
			mapfaceY := mapPicOfY[index]
			if len(mapPicOfX) > 1 {
				index = rand.Intn(len(mapPicOfX))
				mapfaceX = mapPicOfX[index]
				mapfaceY = mapPicOfY[index]
				mapPicOfX = append(mapPicOfX[:index], mapPicOfX[index+1:]...)
				mapPicOfY = append(mapPicOfY[:index], mapPicOfY[index+1:]...)
			}
			for x := 0; x < w; x++ {
				for y := 0; y < h; y++ {
					a := dst.Image().At(mapfaceX*w+x, mapfaceY*h+y)
					cc := color.NRGBAModel.Convert(a).(color.NRGBA)
					returnpic.Image().Set(i*w+x, j*h+y, cc)
				}
			}
		}
	}
	return imgfactory.ToBytes(returnpic.Image())
}

// 拼接提示词
func getTips(cardData cardInfo, quitCount int) string {
	nameStr := strings.ReplaceAll(cardData.CnName, " ", "")
	nameStr = strings.ReplaceAll(nameStr, "-", "")
	nameStr = strings.ReplaceAll(nameStr, "·", "")
	name := []rune(nameStr)
	switch quitCount {
	case 0:
		typeInfo, _, _ := strings.Cut(cardData.Text.Types, "]")
		return "这是一张" + typeInfo + "],卡名是" + strconv.Itoa(len(name)) + "字的"
	case 2:
		if len(name) <= 1 {
			return "这是一张" + cardData.Text.Types
		}
		return "卡名含有: " + string(name[rand.Intn(len(name))])
	default:
		text := cardData.Text.Desc + cardData.Text.Pdesc
		textrand := []string{cardData.Text.Types}
		listmax := regexp.MustCompile(`(「.+」)`).FindAllStringSubmatch(text, -1)
		for _, value := range listmax {
			text = strings.ReplaceAll(text, value[0], "「xxx」")
		}
		depictLines := strings.Split(text, "\n")
		for _, depicts := range depictLines {
			depicts = strings.ReplaceAll(depicts, "\n", "")
			depict := strings.Split(depicts, "。")
			for _, value := range depict {
				value = strings.TrimSpace(value)
				if value != "" {
					list := strings.Split(value, "，")
					for _, value2 := range list {
						if value2 != "" {
							textrand = append(textrand, value2)
						}
					}
				}
			}
		}
		return textrand[rand.Intn(len(textrand))]
	}
}
