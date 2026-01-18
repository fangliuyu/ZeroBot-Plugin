// Package ygo 一些关于ygo的插件
package ygo

import (
	"errors"
	"image/color"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FloatTech/AnimeAPI/wallet"
	"github.com/FloatTech/floatbox/file"
	zbmath "github.com/FloatTech/floatbox/math"
	"github.com/FloatTech/floatbox/process"
	"github.com/FloatTech/imgfactory"
	ctrl "github.com/FloatTech/zbpctrl"
	control "github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension/single"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/FloatTech/gg"
	"github.com/sergi/go-diff/diffmatchpatch"
)

const (
	edgeThreshold   = 225               // 15^2 = 225
	gameDuration    = 120 * time.Second // 单局游戏时长
	warningDuration = 105 * time.Second // 警告时间
	cooldownPeriod  = 30 * time.Minute  // 冷却时间
	maxGameDuration = 1 * time.Hour     // 游戏最大持续时间
)

// GameInfo 游戏信息
type GameInfo struct {
	MID         any
	UID         int64
	CID         int
	Name        []string
	Pic         []byte
	Info        []string
	LastTime    time.Time // 距离上次回答时间
	Worry       int       // 错误次数
	TickCount   int       // 提示次数
	AnswerCount int       // 问答次数
}

type GameLimit struct {
	Limit       int
	LastTime    time.Time
	GameStart   time.Time
	CooldownEnd time.Time
}

// type SafeGameRoom struct {
// 	mu    sync.RWMutex
// 	games map[int64]*GameInfo
// 	limit map[int64]*GameLimit
// }

var (
	nameList   = []string{"CN卡名", "NW卡名", "MD卡名", "简中卡名", "日文注音", "日文名", "英文名"}
	processors = []func(*imgfactory.Factory) ([]byte, error){
		backPic, mosaic, doublePicture, cutPic, randSet,
	}
	gameRoom  sync.Map
	gameCheck sync.Map
	ygoguess  = control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "游戏王猜卡游戏",
		Help:             "- 猜卡游戏\n- 我猜xxx",
	}).ApplySingle(single.New(
		single.WithKeyFn(func(ctx *zero.Ctx) int64 {
			return ctx.Event.GroupID
		}),
		single.WithPostFn[int64](func(ctx *zero.Ctx) {
			ctx.Break()
			text := ctx.ExtractPlainText()
			switch {
			case text == "猜卡游戏":
				ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("已经有正在进行的游戏...")))
			case strings.HasPrefix(text, "我猜"):
				ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("你抢答慢了")))
			case text == "提示":
				ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("处理其他事件中,请稍后重试")))
			}
		}),
	))
)

func init() {
	go func() {
		process.GlobalInitMutex.Lock()
		var ctx *zero.Ctx
		zero.RangeBot(func(id int64, _ *zero.Ctx) bool {
			ctx = zero.GetBot(id)
			return true
		})
		process.GlobalInitMutex.Unlock()

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			gameRoom.Range(func(key, value any) bool {
				gid := key.(int64)
				info := value.(GameInfo)
				sin := time.Since(info.LastTime)
				switch {
				case sin > gameDuration:
					gameRoom.Delete(gid)
					_ = wallet.InsertWalletOf(info.UID, -5)
					picPath := cachePath + strconv.Itoa(info.CID) + ".jpg"
					pic, err := os.ReadFile(picPath)
					if err != nil {
						ctx.SendChain(message.Text("[ERROR]", err))
						return true
					}
					msgID := ctx.SendGroupMessage(gid, message.ReplyWithMessage(info.MID,
						message.Text("时间超时,游戏结束\n卡名是:\n", info.Name[0], "\n"),
						message.ImageBytes(pic)))
					if msgID == 0 {
						ctx.SendGroupMessage(gid, message.Text("时间超时,游戏结束\n图片发送失败,可能被风控\n卡名是:", info.Name[0]))
					}
				case sin >= 105 && sin < 106:
					ctx.SendGroupMessage(gid, message.Text("还有15s作答时间"))
				}
				return true
			})
		}
	}()
	ygoguess.OnFullMatch("猜卡游戏", zero.OnlyGroup).SetBlock(true).Limit(ctxext.LimitByGroup).Handle(func(ctx *zero.Ctx) {
		gid := ctx.Event.GroupID
		ctxMid := ctx.Event.MessageID
		elapsed, err := checkLimit(gid)
		if err != nil {
			ctx.SendChain(message.Text(err))
			return
		}
		info, ok := gameRoom.Load(gid)
		if ok {
			gameInfo := info.(GameInfo)
			if time.Since(gameInfo.LastTime) < warningDuration {
				ctx.SendChain(message.Text("已经有正在进行的游戏(第", elapsed, "题):"))
				mid := ctx.SendChain(message.ImageBytes(gameInfo.Pic))
				if mid.ID() != 0 {
					ctx.SendChain(message.Text("请回答该图的卡名\n以“我猜xxx”格式回答\n(xxx需包含卡名1/4以上)\n或发“提示”得提示;“取消”结束游戏\n猜卡失败或取消会扣除5", wallet.GetWalletName()))
				}
				return
			}
		}
		score := wallet.GetWalletOf(ctx.Event.UserID)
		if score < 1 {
			ctx.SendChain(message.Reply(ctxMid), message.Text("你的", wallet.GetWalletName(), "不足1,无法开启猜卡游戏。"))
			return
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
			MID: ctxMid,
			UID: ctx.Event.UserID,
			CID: data.ID,
			Name: []string{
				data.CnName,
				data.NwbbsN,
				data.MdName,
				data.CnocgN,
				data.JpRuby,
				data.JpName,
				data.EnName,
			},
			LastTime: time.Now(),
			Pic:      pictrue,
			Info:     []string{getTips(data, 0), getTips(data, 1), getTips(data, 2)},
		}
		gameRoom.Store(gid, gameInfo)
		mid := ctx.SendChain(message.ImageBytes(gameInfo.Pic))
		if mid.ID() != 0 {
			ctx.SendChain(message.Text("(第", elapsed, "题)请回答该图的卡名\n以“我猜xxx”格式回答\n(xxx需包含卡名1/4以上)\n或发“提示”得提示;“取消”结束游戏\n\n无人答出则发起者会扣除5", wallet.GetWalletName()))
		}
	})

	ygoguess.OnRegex("^我猜(.+)$", zero.OnlyGroup).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		gid := ctx.Event.GroupID
		info, ok := gameRoom.Load(gid)
		if !ok {
			return
		}
		gameInfo := info.(GameInfo)
		cardName := removePunctuation(gameInfo.Name[0])
		length := zbmath.Ceil(len([]rune(cardName)), 4)

		mid := ctx.Event.MessageID
		answer := ctx.State["regex_matched"].([]string)[1]
		if len([]rune(removePunctuation(answer))) < length {
			ctx.Send(message.ReplyWithMessage(mid, message.Text("请输入", length, "字以上")))
			return
		}
		gameInfo.AnswerCount++
		index, diff := 0, 0
		for i, cardName := range gameInfo.Name {
			if cardName == "" {
				continue
			}
			diff = matchCard(cardName, answer)
			// println(i, cardName, answer, diff)
			if diff != 0 {
				index = i
				break
			}
		}
		if diff == 0 && gameInfo.AnswerCount >= 6 {
			defer gameRoom.Delete(gid)
			picPath := cachePath + strconv.Itoa(gameInfo.CID) + ".jpg"
			pic, err := os.ReadFile(picPath)
			if err != nil {
				ctx.SendChain(message.Text("次数到了,很遗憾没能猜出来.\n卡名是:", gameInfo.Name[0], "\n[ERROR]", err))
				return
			}
			msgID := ctx.Send(message.ReplyWithMessage(mid,
				message.Text("次数到了,很遗憾没能猜出来\n卡名是:\n", gameInfo.Name[0], "\n"),
				message.ImageBytes(pic)))
			if msgID.ID() == 0 {
				ctx.SendChain(message.Text("次数到了,很遗憾没能猜出来\n图片发送失败,可能被风控\n卡名是:", gameInfo.Name[0]))
			}
			return
		}
		if diff == 0 {
			gameInfo.Worry++
			ctx.Send(message.ReplyWithMessage(mid, message.Text("答案不对哦,还有"+strconv.Itoa(6-gameInfo.AnswerCount)+"次回答机会,加油啊~")))
			gameRoom.Store(gid, gameInfo)
			return
		}
		txt := ""
		if diff > 45 {
			sin := time.Since(gameInfo.LastTime).Minutes()
			getMoney := (10 - gameInfo.Worry - gameInfo.TickCount - 2*int(sin)) * (diff - 45) / 45
			if getMoney > 0 {
				err := wallet.InsertWalletOf(ctx.Event.UserID, getMoney)
				if err == nil {
					txt = "\n回答正确,奖励" + strconv.Itoa(getMoney) + wallet.GetWalletName()
				}
			}
		}
		defer gameRoom.Delete(gid)
		anserName := gameInfo.Name[0]
		if index != 0 {
			anserName = "CN译名: " + gameInfo.Name[0] + "\n" + nameList[index] + ": " + gameInfo.Name[index]
		}
		picPath := cachePath + strconv.Itoa(gameInfo.CID) + ".jpg"
		pic, err := os.ReadFile(picPath)
		if err != nil {
			ctx.SendChain(message.Text("太棒了,你猜对了!\n(答案完整度:", diff, "%)", txt, "\n卡名是:", anserName, "\n[ERROR]", err))
			return
		}
		msgID := ctx.Send(message.ReplyWithMessage(mid,
			message.Text("太棒了,你猜对了!\n(答案完整度:", diff, "%)", txt, "\n卡名是:\n", anserName, "\n"),
			message.ImageBytes(pic)))
		if msgID.ID() == 0 {
			ctx.SendChain(message.Text("太棒了,你猜对了!\n(答案完整度:", diff, "%)", txt, "\n图片发送失败,可能被风控\n卡名是:", anserName))
		}
	})
	ygoguess.OnFullMatch("提示", zero.OnlyGroup).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		gid := ctx.Event.GroupID
		info, ok := gameRoom.Load(gid)
		if !ok {
			return
		}
		gameInfo := info.(GameInfo)
		msgID := ctx.Event.MessageID
		if gameInfo.TickCount > 2 {
			ctx.Send(message.ReplyWithMessage(msgID, message.Text("已经没有提示了哦,加油啊")))
			return
		}
		ctx.Send(message.ReplyWithMessage(msgID, message.Text(gameInfo.Info[gameInfo.TickCount])))
		gameInfo.TickCount++
		gameRoom.Store(gid, gameInfo)
	})
	ygoguess.OnFullMatch("取消", zero.OnlyGroup).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		gid := ctx.Event.GroupID
		info, ok := gameRoom.Load(gid)
		if !ok {
			return
		}
		mid := ctx.Event.MessageID
		gameInfo := info.(GameInfo)
		if ctx.Event.UserID != gameInfo.UID {
			ctx.Send(message.ReplyWithMessage(mid, message.Text("你无权限取消")))
			return
		}
		defer gameRoom.Delete(gid)
		_ = wallet.InsertWalletOf(gameInfo.UID, -5)
		picPath := cachePath + strconv.Itoa(gameInfo.CID) + ".jpg"
		pic, err := os.ReadFile(picPath)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]", err))
			return
		}
		gameInfo.Worry = max(gameInfo.Worry, 6)
		msgID := ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID,
			message.Text("游戏已取消\n卡名是:\n", gameInfo.Name[0], "\n"),
			message.ImageBytes(pic)))
		if msgID.ID() == 0 {
			ctx.SendChain(message.Text("游戏已取消\n图片发送失败,可能被风控\n答案是:", gameInfo.Name[0]))
		}
	})
}

// 随机选择
func randPicture(picFile, cardType string) ([]byte, error) {
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
	processor := processors[rand.Intn(len(processors))]
	return processor(dst)
}

// 提取为独立的边缘检测函数
func isEdgePixel(colorA, colorB, colorC color.NRGBA) bool {
	// 计算与右边像素的色差
	diffRight := colorDiffSquared(colorA, colorB)
	if diffRight > edgeThreshold {
		return true
	}

	// 计算与下边像素的色差
	diffDown := colorDiffSquared(colorA, colorC)
	return diffDown > edgeThreshold
}

// 优化色差计算（避免平方根）
func colorDiffSquared(c1, c2 color.NRGBA) int {
	dr := int(c1.R) - int(c2.R)
	dg := int(c1.G) - int(c2.G)
	db := int(c1.B) - int(c2.B)
	return dr*dr + dg*dg + db*db
}

// 获取黑边
func backPic(dst *imgfactory.Factory) ([]byte, error) {
	bounds := dst.Image().Bounds()
	returnpic := imgfactory.NewFactoryBG(dst.W(), dst.H(), color.NRGBA{255, 255, 255, 255}).Image()

	// 避免边界检查
	maxX := bounds.Max.X - 1
	maxY := bounds.Max.Y - 1

	for y := bounds.Min.Y; y < maxY; y++ {
		for x := bounds.Min.X; x < maxX; x++ {
			a := dst.Image().At(x, y)
			colorA := color.NRGBAModel.Convert(a).(color.NRGBA)
			b := dst.Image().At(x+1, y)
			colorB := color.NRGBAModel.Convert(b).(color.NRGBA)
			c := dst.Image().At(x, y+1)
			colorC := color.NRGBAModel.Convert(c).(color.NRGBA)

			if isEdgePixel(colorA, colorB, colorC) {
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
	w, h := b.Dx(), b.Dy()

	// 根据图片大小动态调整马赛克块大小
	blockSize := max(w, h) / 30
	blockSize = max(blockSize, 4) // 最小4像素

	// 预计算马赛克块
	xBlocks := (w + blockSize - 1) / blockSize
	yBlocks := (h + blockSize - 1) / blockSize

	// 使用缓存颜色提高性能
	colors := make([][]color.NRGBA, xBlocks)
	for i := range colors {
		colors[i] = make([]color.NRGBA, yBlocks)
	}

	// 采样每个块的中心颜色
	for x := 0; x < xBlocks; x++ {
		for y := 0; y < yBlocks; y++ {
			cx := x*blockSize + blockSize/2
			cy := y*blockSize + blockSize/2
			if cx >= w {
				cx = w - 1
			}
			if cy >= h {
				cy = h - 1
			}
			colors[x][y] = color.NRGBAModel.Convert(
				dst.Image().At(cx, cy)).(color.NRGBA)
		}
	}

	// 应用马赛克
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			bx := x / blockSize
			by := y / blockSize
			dst.Image().Set(x, y, colors[bx][by])
		}
	}

	return imgfactory.ToBytes(dst.Blur(2).Image())
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
	w, h := b.Max.X, b.Max.Y
	blockW, blockH := w/3, h/3

	// 创建输出图像
	returnpic := imgfactory.NewFactoryBG(w, h, color.NRGBA{255, 255, 255, 255})

	// 生成9个块的随机排列
	indices := rand.Perm(9)
	// 预计算步长
	srcStride := dst.Image().Stride
	dstStride := returnpic.Image().Stride

	// 逐块复制像素
	for i := range 9 {
		srcIdx := indices[i]

		// 计算源块和目标块的坐标
		srcBlockX := (srcIdx % 3) * blockW
		srcBlockY := (srcIdx / 3) * blockH
		dstBlockX := (i % 3) * blockW
		dstBlockY := (i / 3) * blockH

		// 复制整个块
		for y := range blockH {
			srcY := srcBlockY + y
			dstY := dstBlockY + y

			if srcY >= h || dstY >= h {
				break
			}

			// 计算行起始位置
			srcStart := srcY*srcStride + srcBlockX*4
			dstStart := dstY*dstStride + dstBlockX*4

			// 整行复制（使用 copy 函数）
			copy(
				returnpic.Image().Pix[dstStart:dstStart+blockW*4],
				dst.Image().Pix[srcStart:srcStart+blockW*4],
			)
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
		listmax := regexp.MustCompile(`(「[^」]*」)`).FindAllStringSubmatch(text, -1)
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
					for value2 := range strings.SplitSeq(value, "，") {
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

func matchCard(cardName, text string) int {
	an := removePunctuationAndLower(text)
	cn := removePunctuationAndLower(cardName)

	if an == "" || cn == "" {
		return 0
	}

	// 优先检查完全匹配或包含关系
	if an == cn {
		return 100
	}

	if strings.Contains(cn, an) {
		return len([]rune(an)) * 100 / len([]rune(cn))
	}

	// 使用更高效的字符串相似度算法
	similarity := calculateSimilarity(cn, an)

	// 设置相似度阈值（75%）
	if similarity >= 75 {
		return similarity
	}

	return 0
}

// 实现Levenshtein距离算法
func calculateSimilarity(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)

	if n == 0 || m == 0 {
		return 0
	}

	// 优化：如果长度差太大，直接返回0
	if zbmath.Abs(n-m) > max(n, m)/2 {
		return 0
	}
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(s1, s2, false)
	matched := 0
	for _, diff := range diffs {
		if diff.Type == diffmatchpatch.DiffEqual {
			matched += len([]rune(diff.Text))
		}
	}
	lenght := max(n, m)
	if matched >= lenght*3/4 {
		return matched * 100 / lenght
	}
	return 0
}

func removePunctuationAndLower(s string) string {
	return strings.ToLower(removePunctuation(s))
}

func removePunctuation(text string) string {
	punctuations := ` ·~!@#$%^&*()-_+={}[]|\;:"<>,./?`
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(punctuations, r) {
			return -1
		}
		return r
	}, text)
}

// 修改 checkLimit 函数
func checkLimit(gid int64) (int, error) {
	info, ok := gameCheck.Load(gid)
	if !ok {
		gameLimit := GameLimit{
			Limit:     1,
			LastTime:  time.Now(),
			GameStart: time.Now(),
		}
		gameCheck.Store(gid, gameLimit)
		return 1, nil
	}

	gameLimit := info.(GameLimit)

	// 清除冷却状态
	if !gameLimit.CooldownEnd.IsZero() && time.Now().After(gameLimit.CooldownEnd) {
		gameLimit.CooldownEnd = time.Time{}
		gameLimit.GameStart = time.Now()
	}

	// 检查冷却状态
	if !gameLimit.CooldownEnd.IsZero() && time.Now().Before(gameLimit.CooldownEnd) {
		return gameLimit.Limit, errors.New("冷却时间中，剩余" +
			time.Until(gameLimit.CooldownEnd).Round(time.Second).String())
	}

	// 检查是否达到1小时限制
	if !gameLimit.GameStart.IsZero() && time.Since(gameLimit.GameStart) >= maxGameDuration {
		gameCheck.Store(gid, GameLimit{
			LastTime:    time.Now(),
			CooldownEnd: time.Now().Add(cooldownPeriod),
		})
		return gameLimit.Limit, errors.New("游戏时间已达1小时，进入半小时冷却")
	}

	// 检查是否是新的一天
	now := time.Now()
	if now.Day() != gameLimit.LastTime.Day() {
		gameLimit.Limit = 1
		gameLimit.GameStart = now
	} else {
		gameLimit.Limit++
	}

	gameLimit.LastTime = now
	gameCheck.Store(gid, gameLimit)

	return gameLimit.Limit, nil
}
