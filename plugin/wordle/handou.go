// Package wordle 猜单词
package wordle

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"maps"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/FloatTech/imgfactory"
	"github.com/sirupsen/logrus"

	fcext "github.com/FloatTech/floatbox/ctxext"
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/gg"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/FloatTech/zbputils/img/text"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

type idiomJson struct {
	Derivation   string `json:"derivation"`
	Example      string `json:"example"`
	Explanation  string `json:"explanation"`
	Pinyin       string `json:"pinyin"`
	Word         string `json:"word"`
	Abbreviation string `json:"abbreviation"`
}

type idiomInfo struct {
	Pinyin      []string `json:"pinyin"`      // 拼音
	Derivation  string   `json:"derivation"`  // 词源
	Explanation string   `json:"explanation"` // 解释
}

// GameInfo 游戏信息
type GameInfo struct {
	Personal    bool // 是否个人游戏
	MID         any
	UID         int64
	World       []rune     // 成语
	WorldLength int        // 成语长度
	Record      [][]rune   // 猜测记录
	Tick        [][]string // 拼音提示
	LastTime    time.Time  // 距离上次回答时间
	Worry       int        // 错误次数
	TickCount   int        // 提示次数
	AnswerCount int        // 问答次数
}

var (
	kong           rune = ' '
	pinFontSize         = 45.0
	hanFontSize         = 150.0
	pinyinFont     []byte
	idiomInfoMap   = make(map[string]idiomInfo)
	userHabitsFile = file.BOTPATH + "/" + en.DataFolder() + "userHabits.json"
	mu             sync.Mutex
	habits         = make(map[string]int)
)

func init() {
	en.OnRegex(`^(个人|团队)猜成语$`, zero.OnlyGroup, fcext.DoOnceOnSuccess(
		func(ctx *zero.Ctx) bool {
			_, err := en.GetLazyData("idiom.json", true)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: 下载字典时发生错误.\n", err))
				return false
			}
			idiomFile, err := os.ReadFile(file.BOTPATH + "/" + en.DataFolder() + "idiom.json")
			if err != nil {
				ctx.SendChain(message.Text("ERROR: 读取字典时发生错误.\n", err))
				return false
			}
			var idiomFileJson []idiomJson
			err = json.Unmarshal(idiomFile, &idiomFileJson)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: 解析字典时发生错误.\n", err))
				return false
			}
			for _, v := range idiomFileJson {
				idiomInfoMap[v.Word] = idiomInfo{
					Pinyin:      strings.Split(v.Pinyin, " "),
					Derivation:  v.Derivation,
					Explanation: v.Explanation,
				}
			}
			// 构建用户习惯库（全局高频N-gram）
			if file.IsNotExist(userHabitsFile) {
				f, err := os.Create(userHabitsFile)
				if err != nil {
					ctx.SendChain(message.Text("ERROR: 创建用户习惯库时发生错误.\n", err))
					return false
				}
				_ = f.Close()
			} else {
				habitsFile, err := os.ReadFile(userHabitsFile)
				if err != nil {
					ctx.SendChain(message.Text("ERROR: 读取字典时发生错误.\n", err))
					return false
				}
				var config = make(map[string]int)
				err = json.Unmarshal(habitsFile, &config)
				if err != nil {
					ctx.SendChain(message.Text("ERROR: 解析字典时发生错误.\n", err))
					return false
				}
				habits = buildUserHabits(config)
			}
			data, err := file.GetLazyData(text.BoldFontFile, control.Md5File, true)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: 解析字典时发生错误.\n", err))
				return false
			}
			pinyinFont = data
			return true
		},
	)).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		target := poolIdiom()
		chengyu := []rune(target)
		tt := idiomInfoMap[target].Pinyin[0]
		worldLength := len(chengyu)
		game := newHandouGame(chengyu)
		anser := anserOutString(target)
		_, img, _ := game("")
		ctx.Send(
			message.ReplyWithMessage(ctx.Event.MessageID,
				message.ImageBytes(img),
				message.Text("你有", 7, "次机会猜出", worldLength, "字成语\n首字拼音为：", tt),
			),
		)
		var next *zero.FutureEvent
		if ctx.State["regex_matched"].([]string)[1] == "个人" {
			next = zero.NewFutureEvent("message", 999, false, zero.RegexRule(fmt.Sprintf(`^([\p{Han}，,]){%d}$`, worldLength)),
				zero.OnlyGroup, ctx.CheckSession())
		} else {
			next = zero.NewFutureEvent("message", 999, false, zero.RegexRule(fmt.Sprintf(`^([\p{Han}，,]){%d}$`, worldLength)),
				zero.OnlyGroup, zero.CheckGroup(ctx.Event.GroupID))
		}
		var err error
		var win bool
		recv, cancel := next.Repeat()
		defer cancel()
		tick := time.NewTimer(105 * time.Second)
		after := time.NewTimer(120 * time.Second)
		for {
			select {
			case <-tick.C:
				ctx.SendChain(message.Text("猜成语，你还有15s作答时间"))
			case <-after.C:
				ctx.Send(
					message.ReplyWithMessage(ctx.Event.MessageID,
						message.Text("猜成语超时，游戏结束...\n答案是: ", anser),
					),
				)
				return
			case c := <-recv:
				tick.Reset(105 * time.Second)
				after.Reset(120 * time.Second)
				err = updateHabits(c.Event.Message.String())
				if err != nil {
					logrus.Warn("更新用户习惯库时发生错误: ", err)
				}
				win, img, err = game(c.Event.Message.String())
				switch {
				case win:
					tick.Stop()
					after.Stop()
					ctx.Send(
						message.ReplyWithMessage(c.Event.MessageID,
							message.ImageBytes(img),
							message.Text("太棒了，你猜出来了！\n答案是: ", anser),
						),
					)
					return
				case err == errTimesRunOut:
					tick.Stop()
					after.Stop()
					ctx.Send(
						message.ReplyWithMessage(c.Event.MessageID,
							message.ImageBytes(img),
							message.Text("游戏结束...\n答案是: ", anser),
						),
					)
					return
				case err == errLengthNotEnough:
					ctx.Send(
						message.ReplyWithMessage(c.Event.MessageID,
							message.Text("成语长度错误"),
						),
					)
				case err == errHadGuessed:
					ctx.Send(
						message.ReplyWithMessage(c.Event.MessageID,
							message.Text("该成语已经猜过了"),
						),
					)
				case err == errUnknownWord:
					ctx.Send(
						message.ReplyWithMessage(c.Event.MessageID,
							message.Text("你确定存在这样的成语吗？"),
						),
					)
				default:
					ctx.Send(
						message.ReplyWithMessage(c.Event.MessageID,
							message.ImageBytes(img),
						),
					)
				}
			}
		}
	})
}

func poolIdiom() string {
	prioritizedData := prioritizeData(idiomInfoMap)
	if len(prioritizedData) > 0 {
		return prioritizedData[rand.Intn(len(prioritizedData))]
	}
	// 如果没有优先级数据，则随机选择一个成语
	keys := make([]string, 0, len(idiomInfoMap))
	for k := range idiomInfoMap {
		keys = append(keys, k)
	}
	return keys[rand.Intn(len(keys))]
}

// 保存用户配置
func saveConfig(data map[string]int) error {
	mu.Lock()
	defer mu.Unlock()
	if reader, err := os.Create(userHabitsFile); err == nil {
		err = json.NewEncoder(reader).Encode(&data)
		if err != nil {
			return err
		}
	} else {
		return err
	}
	return nil
}

// 统计N-gram频率
func countNGrams(input string) map[string]int {
	ngrams := make(map[string]int)
	words := []rune(input)
	for i := range words {
		ngrams[string(words[i])]++
	}
	return ngrams
}

func updateHabits(input string) error {
	descriptionNGrams := countNGrams(input) // 假设用2-gram匹配
	maps.Copy(habits, descriptionNGrams)
	return saveConfig(habits)
}

// 构建用户习惯库
func buildUserHabits(inputs map[string]int) map[string]int {
	habits := make(map[string]int)
	for word := range inputs {
		ngrams := countNGrams(word)
		for ngram, count := range ngrams {
			habits[ngram] += count
		}
	}
	return habits
}

// 优先抽取包含高频N-gram的数据
func prioritizeData(data map[string]idiomInfo) []string {
	var prioritized []string
	for world := range data {
		descriptionNGrams := countNGrams(world) // 假设用2-gram匹配
		score := 0
		for ngram, count := range descriptionNGrams {
			if habitCount, exists := habits[ngram]; exists {
				score += habitCount * count // 权重可调整
			}
		}
		if score > len(habits) { // 仅保留匹配到高频N-gram的数据
			prioritized = append(prioritized, world)
		}
		if len(prioritized) > 9 {
			break
		}
	}
	return prioritized
}

func newHandouGame(target []rune) func(string) (bool, []byte, error) {
	var class = len(target)
	tt := idiomInfoMap[string(target)].Pinyin
	record := make([]string, 0, 7)
	tick := make([][]string, class)
	tickhanByte := make([]string, class)

	// 初始化 tick，确保 tick[i][0] 和 tick[i][1] 有正确的初始值
	for i := range class {
		tick[i] = make([]string, 2)
		if i == 0 {
			tick[i][0] = tt[0] // 初始化为目标拼音
		} else {
			tick[i][0] = "" // 防止越界
		}
		tick[i][1] = "?" // 初始化为空
	}

	return func(s string) (win bool, data []byte, err error) {
		answer := []rune(s)
		var answerData idiomInfo

		if s != "" {
			if string(target) == s {
				win = true
			}

			if len(answer) != len(target) {
				err = errLengthNotEnough
				return
			}
			for _, v := range record {
				if s == v {
					err = errHadGuessed
					return
				}
			}

			answerInfo, ok := idiomInfoMap[s]
			if !ok {
				err = errUnknownWord
				return
			}
			answerData = answerInfo

			// 处理拼音匹配逻辑
			for i := 0; i < class && i < len(answerData.Pinyin) && i < len(tt); i++ {
				tp := []rune(tt[i])
				tickPinByte := make([]rune, len(tp))
				if i < len(tick) && tick[i][0] != "" {
					copy(tickPinByte, []rune(tick[i][0]))
				} else {
					for k := range tickPinByte {
						tickPinByte[k] = kong
					}
				}

				v := answerData.Pinyin[i]
				for j, w := range []rune(v) {
					if j >= len(tp) {
						break
					}
					switch {
					case w == tp[j]:
						tickPinByte[j] = w
						if strings.Contains(tickhanByte[i], string(w)) {
							tickhanByte[i] = strings.ReplaceAll(tickhanByte[i], string(w), "")
						}
					case strings.Contains(tt[i], string(w)):
						if strings.Contains(tickhanByte[i], string(w)) {
							continue
						}
						tickhanByte[i] += string(w)
					default:
						if tickPinByte[j] != kong {
							continue
						}
						tickPinByte[j] = kong
					}
				}
				matchIndex := -1
				for j, v := range tickPinByte {
					if v != kong && v != '_' {
						matchIndex = j
					}
				}
				for j := range tickPinByte {
					if j > matchIndex {
						break
					}
					if tickPinByte[j] == kong {
						tickPinByte[j] = '_'
					}
				}
				tick[i][0] = string(tickPinByte)
			}

			// 处理汉字匹配逻辑
			for i := 0; i < class && i < len(answer); i++ {
				if i < len(tick) && answer[i] == target[i] {
					tick[i][1] = string(target[i])
				}
			}

			record = append(record, s)
			if len(record) > 3 && len(tick) > 0 {
				tick[0][1] = string(target[0])
			}
		}

		// 准备绘制数据
		tickHan := make([]rune, 0, class)
		tickPin := make([]string, 0, class)
		for i := 0; i < class && i < len(tick); i++ {
			if i < len(tick) {
				tickHan = append(tickHan, []rune(tick[i][1])...)
				tickPin = append(tickPin, tick[i][0])
			}
		}
		if s == "" {
			// 空输入处理
			answer = tickHan
			answerData.Pinyin = tickPin
		}

		var (
			tickImage   image.Image
			answerImage image.Image
			imgHistery  = make([]image.Image, 0, 7)
			hisH        = 0
			wg          = &sync.WaitGroup{}
		)
		wg.Add(2)

		go func() {
			defer wg.Done()
			tickImage = drawHanBloack(hanFontSize/2, pinFontSize/2, tickHan, tickPin, target, tt)
		}()
		go func() {
			defer wg.Done()
			answerImage = drawHanBloack(hanFontSize, pinFontSize, answer, answerData.Pinyin, target, tt)
		}()
		if len(record) > 1 {
			wg.Add(len(record) - 1)
			for i, v := range record[:len(record)-1] {
				imgHistery = append(imgHistery, nil)
				go func(i int, v string) {
					defer wg.Done()
					pin := idiomInfoMap[v].Pinyin
					han := []rune(v)
					hisImage := drawHanBloack(hanFontSize/3, pinFontSize/3, han, pin, target, tt)
					imgHistery[i] = hisImage
					if i == 0 {
						hisH = hisImage.Bounds().Dy()
					}
				}(i, v)
			}
		}
		wg.Wait()

		tickW, tickH := tickImage.Bounds().Dx(), tickImage.Bounds().Dy()
		answerW, answerH := answerImage.Bounds().Dx(), answerImage.Bounds().Dy()

		ctx := gg.NewContext(1, 1)
		_ = ctx.ParseFontFace(pinyinFont, pinFontSize/2)
		wordH, _ := ctx.MeasureString("M")

		ctxWidth := answerW
		ctxHeight := tickH + answerH + int(wordH) + hisH*(len(imgHistery)+1)/2

		ctx = gg.NewContext(ctxWidth, ctxHeight)
		ctx.SetColor(color.RGBA{255, 255, 255, 255})
		ctx.Clear()

		ctx.SetColor(color.RGBA{0, 0, 0, 255})
		_ = ctx.ParseFontFace(pinyinFont, hanFontSize/2)
		ctx.DrawStringAnchored("题目:", float64(ctxWidth-tickW)/4, float64(tickH)/2, 0.5, 0.5)

		ctx.DrawImageAnchored(tickImage, ctxWidth/2, tickH/2, 0.5, 0.5)
		ctx.DrawImageAnchored(answerImage, ctxWidth/2, tickH+int(wordH)+answerH/2, 0.5, 0.5)

		x := float64(ctxWidth-tickW)/2 + float64(tickW)/3
		_ = ctx.ParseFontFace(pinyinFont, pinFontSize/2)
		ctx.SetColor(color.RGBA{255, 128, 0, 255})
		for _, v := range tickhanByte[1:] {
			if v != "" {
				v = "?" + v
				_, wordW := ctx.MeasureString("M")
				ctx.DrawStringAnchored(v, x+wordW/2, float64(tickH)+wordH/2, 0.5, 0.5)
			}
			x += float64(tickW) / 4
		}

		k := 0
		for i, v := range imgHistery {
			if v == nil {
				break // 如果没有历史记录，跳过
			}
			x := ctxWidth / 4

			y := tickH + int(wordH) + answerH + hisH*k
			if i%2 == 1 {
				x = ctxWidth * 3 / 4
				k++
			}
			ctx.DrawImageAnchored(v, x, y+hisH/2, 0.5, 0.5)
		}

		data, err = imgfactory.ToBytes(ctx.Image())
		if len(record) >= cap(record) {
			err = errTimesRunOut
			return
		}
		return
	}
}

func drawHanBloack(hanFontSize, pinFontSize float64, han []rune, pinyin []string, answer []rune, answerPinyin []string) image.Image {
	class := len(answer) // 以 answer 的长度为准
	if len(han) < class {
		// 补全 han
		tmp := make([]rune, class)
		copy(tmp, han)
		for i := len(han); i < class; i++ {
			tmp[i] = '?'
		}
		han = tmp
	}
	if len(pinyin) < class {
		// 补全 pinyin
		tmp := make([]string, class)
		copy(tmp, pinyin)
		for i := len(pinyin); i < class; i++ {
			tmp[i] = "?"
		}
		pinyin = tmp
	}
	if len(answerPinyin) < class {
		// 补全 answerPinyin
		tmp := make([]string, class)
		copy(tmp, answerPinyin)
		for i := len(answerPinyin); i < class; i++ {
			tmp[i] = "?"
		}
		answerPinyin = tmp
	}

	ctx := gg.NewContext(1, 1)
	_ = ctx.ParseFontFace(pinyinFont, pinFontSize)
	pinWidth, pinHeight := ctx.MeasureString("M")
	_ = ctx.ParseFontFace(pinyinFont, hanFontSize)
	_, hanHeight := ctx.MeasureString("拼")

	space := int(pinHeight / 2)
	boxPadding := hanHeight * 0.3
	bloackPinWidth := int(pinWidth*5) + space

	ctx = gg.NewContext(
		space+class*bloackPinWidth,
		space+int(pinHeight+hanHeight+boxPadding*2)+space*2,
	)
	ctx.SetColor(color.RGBA{255, 255, 255, 255})
	ctx.Clear()

	for i := range class {
		x := float64(space + i*bloackPinWidth)

		// 绘制拼音
		_ = ctx.ParseFontFace(pinyinFont, pinFontSize)
		if i < len(pinyin) {
			pinyinByte := []rune(pinyin[i])
			pinTotalWidth := pinWidth * float64(len(pinyinByte))
			pinX := x + float64(bloackPinWidth)/2 - pinTotalWidth/2
			pinY := float64(space) + pinHeight/2

			for k, ch := range pinyinByte {
				ctx.SetColor(colors[notexist])
				if i < len(answerPinyin) {
					targetByte := []rune(answerPinyin[i])
					for m, targetCh := range targetByte {
						if k == m && ch == targetCh {
							ctx.SetColor(color.RGBA{0, 153, 0, 255})
							break
						} else if strings.ContainsRune(answerPinyin[i], ch) {
							ctx.SetColor(color.RGBA{255, 128, 0, 255})
						}
					}
				}
				ctx.DrawStringAnchored(string(ch), pinX+pinWidth*float64(k)+pinWidth/2, pinY, 0.5, 0.5)
			}
		}

		// 绘制汉字方框
		boxX := x + boxPadding
		boxY := pinHeight + boxPadding
		boxWidth := float64(bloackPinWidth) - boxPadding*2
		boxHeight := float64(hanHeight) + boxPadding*2
		ctx.DrawRectangle(boxX, boxY, boxWidth, boxHeight)

		// 设置方框颜色
		if i < len(han) && i < len(answer) {
			switch {
			case han[i] == answer[i]:
				ctx.SetColor(colors[match])
			case i < len(answer) && strings.ContainsRune(string(answer), han[i]):
				ctx.SetColor(colors[exist])
			default:
				ctx.SetColor(colors[notexist])
			}
		} else {
			ctx.SetColor(colors[notexist])
		}
		ctx.Fill()

		// 绘制汉字
		_ = ctx.ParseFontFace(pinyinFont, hanFontSize)
		ctx.SetColor(color.RGBA{255, 255, 255, 255})
		if i < len(han) {
			hanX := boxX + boxWidth/2
			hanY := boxY + boxHeight/2
			ctx.DrawStringAnchored(string(han[i]), hanX, hanY, 0.5, 0.5)
		}
	}
	return ctx.Image()
}

func anserOutString(s string) string {
	data, ok := idiomInfoMap[s]
	if !ok {
		return "未知成语"
	}
	return fmt.Sprintf("%s\n词源: %s\n解释: %s",
		s,
		data.Derivation,
		data.Explanation,
	)
}
