// Package ygo 一些关于ygo的插件
package ygo

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"time"

	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/web"
	ctrl "github.com/FloatTech/zbpctrl"
	control "github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/FloatTech/gg"
	"github.com/FloatTech/imgfactory"
	"github.com/FloatTech/zbputils/img/text"
)

const (
	myCardPieAPI    = "https://sapi.moecube.com:444/ygopro/analytics/deck/type?type=%v&source=mycard-%v"
	myCardPlayerAPI = "https://sapi.moecube.com:444/ygopro/arena/user?username="
	ua              = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.93 Safari/537.36"
	width           = 680
	height          = 600
	radius          = 200
	Label1Size      = 18 // 第一行文字大小（标签名）
	Label2Size      = 12 // 第二行文字大小（数值和百分比）
	TitleSize       = 24 // 标题大小
)

type myCardData []struct {
	Name       string    `json:"name"`
	RecentTime time.Time `json:"recent_time"`
	Source     string    `json:"source"`
	Count      string    `json:"count"`
	Tags       []string  `json:"tags"`
	// Matchup    struct {
	// 	First struct {
	// 		Decka string `json:"decka"`
	// 		Win   string `json:"win"`
	// 		Draw  string `json:"draw"`
	// 		Lose  string `json:"lose"`
	// 	} `json:"first"`
	// 	Second struct {
	// 		Deckb string `json:"deckb"`
	// 		Win   string `json:"win"`
	// 		Draw  string `json:"draw"`
	// 		Lose  string `json:"lose"`
	// 	} `json:"second"`
	// } `json:"matchup"`
}

type pieData struct {
	label string
	value int
}

type playerData struct {
	Exp              int    `json:"exp"`
	Pt               int    `json:"pt"`
	EntertainWin     int    `json:"entertain_win"`
	EntertainLose    int    `json:"entertain_lose"`
	EntertainDraw    int    `json:"entertain_draw"`
	EntertainAll     int    `json:"entertain_all"`
	EntertainWlRatio string `json:"entertain_wl_ratio"`
	ExpRank          int    `json:"exp_rank"`
	AthleticWin      int    `json:"athletic_win"`
	AthleticLose     int    `json:"athletic_lose"`
	AthleticDraw     int    `json:"athletic_draw"`
	AthleticAll      int    `json:"athletic_all"`
	AthleticWlRatio  string `json:"athletic_wl_ratio"`
	ArenaRank        int    `json:"arena_rank"`
}

// 存储每个标签的位置和尺寸信息，用于避免重叠
type labelInfo struct {
	labelX, labelY float64
	width, height  float64
}

var (
	typeMap = map[string]string{
		"今日": "day",
		"月度": "month",
		"竞技": "athletic",
		"娱乐": "entertain",
	}
	colors = []struct{ R, G, B float64 }{
		{0.9, 0.3, 0.3}, {0.3, 0.9, 0.3}, {0.3, 0.3, 0.9},
		{0.9, 0.9, 0.3}, {0.9, 0.3, 0.9}, {0.3, 0.9, 0.9},
		{0.9, 0.6, 0.3}, {0.6, 0.3, 0.9}, {0.3, 0.6, 0.9},
		{0.6, 0.6, 0.6}, {0.9, 0.9, 0.9},
	}
	lasttime time.Time
	todayPic = make(map[string][]byte, 4)
	mycard   = control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "萌卡功能",
		Help:             "- 萌卡[今日|月度]饼图\n- 萌卡[今日|月度][竞技|娱乐]饼图\n- 萌卡胜率 玩家名",
	}).ApplySingle(ctxext.GroupSingle)
)

func init() {
	mycard.OnRegex(`^萌卡(今日|月度)?(竞技|娱乐)?饼图$`).SetBlock(true).Limit(ctxext.LimitByGroup).Handle(func(ctx *zero.Ctx) {
		typ := "今日"
		source := "竞技"
		typOp := ctx.State["regex_matched"].([]string)[1]
		sourceOp := ctx.State["regex_matched"].([]string)[2]
		if typOp != "" {
			typ = typOp
		}
		if sourceOp != "" {
			source = sourceOp
		}
		if img, ok := todayPic[typ+source]; ok && time.Since(lasttime) < 12*time.Hour {
			ctx.SendChain(message.ImageBytes(img))
			return
		}
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("正在获取数据，请稍候..."))
		data, err := getMyCardData(typ, source)
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return
		}
		pieDatas, total := calculatePieData(data)
		if total == 0 {
			ctx.SendChain(message.Text("ERROR: 数据解析失败"))
			return
		}
		imgData, err := generatePieChart(pieDatas, typ, source, total)
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return
		}

		lasttime = time.Now()
		todayPic[typ+source] = imgData
		ctx.SendChain(message.ImageBytes(imgData))
	})
	mycard.OnRegex(`^萌卡胜率\s*(.+)$`).SetBlock(true).Limit(ctxext.LimitByGroup).Handle(func(ctx *zero.Ctx) {
		name := ctx.State["regex_matched"].([]string)[1]
		if name == "" {
			ctx.SendChain(message.Text("请输入玩家名称"))
			return
		}
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("正在获取数据，请稍候..."))
		url := myCardPlayerAPI + url.QueryEscape(name)
		data, err := web.RequestDataWith(web.NewDefaultClient(), url, "GET", "https://mycard.world/", ua, nil)
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return
		}
		var player playerData
		err = json.Unmarshal(data, &player)
		if err != nil {
			ctx.SendChain(message.Text("ERROR: 数据处理失败", err))
			return
		}
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(
			name, " 数据如下:\n",
			"竞技排名: ", player.ArenaRank, " [胜率: ", player.AthleticWin, "胜/", player.AthleticAll, "场（", player.AthleticWlRatio, "%)]\n",
			"娱乐排名: ", player.ExpRank, " [胜率: ", player.EntertainWin, "胜/", player.EntertainAll, "场（", player.EntertainWlRatio, "%)]",
		))
	})
}

func getMyCardData(typ, source string) (myCardData, error) {
	url := fmt.Sprintf(myCardPieAPI, typeMap[typ], typeMap[source])
	data, err := web.GetData(url)
	if err != nil {
		return nil, err
	}
	var result myCardData
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func calculatePieData(data myCardData) ([]pieData, int) {
	pieDatas := make([]pieData, 0, len(data))
	var total int
	var otherTotal int
	for i, d := range data {
		value, err := strconv.Atoi(d.Count)
		if err != nil {
			continue
		}
		if i < 10 {
			// 只取前10个数据
			pieDatas = append(pieDatas, pieData{
				label: d.Name,
				value: value,
			})
		} else {
			otherTotal += value
		}
		total += value
	}
	pieDatas = append(pieDatas, pieData{
		label: "other",
		value: otherTotal,
	})
	return pieDatas, total
}

func generatePieChart(pieDatas []pieData, typ, source string, total int) ([]byte, error) {
	canvas := gg.NewContext(width, height)
	canvas.SetRGB(1, 1, 1) // 背景色为白色
	canvas.Clear()

	font, err := file.GetLazyData(text.BoldFontFile, control.Md5File, true)
	if err != nil {
		return nil, err
	}

	startAngle := -math.Pi / 2 // 从顶部开始绘制

	var labels []labelInfo

	for i, d := range pieDatas {
		percentage := float64(d.value) / float64(total)
		angle := percentage * 2 * math.Pi

		color := colors[i%len(colors)]
		canvas.SetRGB(color.R, color.G, color.B)
		canvas.DrawArc(width/2, height/2, radius, startAngle, startAngle+angle)
		canvas.LineTo(width/2, height/2)
		canvas.Fill()

		midAngle := startAngle + angle/2
		labelX := width/2 + (radius+65)*math.Cos(midAngle)
		labelY := height/2 + (radius+20)*math.Sin(midAngle)

		// 分别测量两行文字的尺寸（使用不同字体大小）
		// 第一行文字
		if err = canvas.ParseFontFace(font, Label1Size); err != nil {
			return nil, err
		}
		w1, h1 := canvas.MeasureString(d.label)

		// 第二行文字
		if err = canvas.ParseFontFace(font, Label2Size); err != nil {
			return nil, err
		}
		labelText2 := fmt.Sprintf("%d (%.2f%%)", d.value, percentage*100)
		w2, h2 := canvas.MeasureString(labelText2)

		// 计算整体标签尺寸
		labelWidth := math.Max(w1, w2)
		labelHeight := h1 + h2 // 两行间距8px

		// 调整位置避免重叠
		adjustedX, adjustedY := adjustLabelPosition(labelX, labelY, labelWidth, labelHeight, labels, width, height)

		// 存储标签信息
		labels = append(labels, labelInfo{
			labelX: adjustedX,
			labelY: adjustedY,
			width:  labelWidth,
			height: labelHeight,
		})

		// 绘制第一行文字（标签名）- 较大字体
		if err = canvas.ParseFontFace(font, Label1Size); err != nil {
			return nil, err
		}
		canvas.SetRGB(0, 0, 0) // 黑色文字
		canvas.DrawStringAnchored(d.label, adjustedX, adjustedY, 0.5, 0)

		// 绘制第二行文字（数值和百分比）- 较小字体
		if err = canvas.ParseFontFace(font, Label2Size); err != nil {
			return nil, err
		}
		canvas.DrawStringAnchored(labelText2, adjustedX, adjustedY+h1, 0.5, 0)

		startAngle += angle
	}

	// 绘制标题（使用标题字体大小）
	if err = canvas.ParseFontFace(font, TitleSize); err != nil {
		return nil, err
	}
	_, textH := canvas.MeasureString("M")
	canvas.DrawStringAnchored("MyCard "+typ+source+"饼图", float64(width)/2, 10+textH, 0.5, 0.5)

	// 绘制时间（使用较小的字体）
	if err = canvas.ParseFontFace(font, Label2Size); err != nil {
		return nil, err
	}
	canvas.DrawStringAnchored("获取时间: "+time.Now().Format("2006-01-02 15:04:05"), float64(width)/2, float64(height)-10-textH, 0.5, 0.5)

	// 生成图片
	return imgfactory.ToBytes(canvas.Image())
}

// 调整标签位置避免重叠
func adjustLabelPosition(x, y, width, height float64, existingLabels []labelInfo, canvasWidth, canvasHeight float64) (float64, float64) {
	const padding = 5.0

	// 计算标签的边界框
	left := x - width/2
	right := x + width/2
	top := y
	bottom := y + height

	// 检查边界，确保在画布内
	if left < padding {
		x = padding + width/2
	}
	if right > float64(canvasWidth)-padding {
		x = float64(canvasWidth) - padding - width/2
	}
	if top < padding {
		y = padding
	}
	if bottom > float64(canvasHeight)-padding {
		y = float64(canvasHeight) - padding - height
	}

	// 重新计算边界框
	left = x - width/2
	right = x + width/2
	top = y
	bottom = y + height

	// 检查与其他标签的重叠
	maxIterations := 50
	for range maxIterations {
		overlap := false

		for _, existing := range existingLabels {
			existingLeft := existing.labelX - existing.width/2
			existingRight := existing.labelX + existing.width/2
			existingTop := existing.labelY
			existingBottom := existing.labelY + existing.height

			// 检查矩形重叠
			if !(right < existingLeft || left > existingRight || bottom < existingTop || top > existingBottom) {
				// 有重叠，调整位置
				overlap = true

				// 计算移动方向（优先垂直移动）
				centerY := (top + bottom) / 2
				existingCenterY := (existingTop + existingBottom) / 2

				if centerY < existingCenterY {
					// 向上移动
					newY := existingTop - height - padding
					if newY < padding {
						// 如果向上会超出边界，改为向下移动
						newY = existingBottom + padding
					}
					y = newY
				} else {
					// 向下移动
					newY := existingBottom + padding
					if newY+height > float64(canvasHeight)-padding {
						// 如果向下会超出边界，改为向上移动
						newY = existingTop - height - padding
						if newY < padding {
							// 如果向上也会超出，则水平移动
							if x < existing.labelX {
								x = existingLeft - width/2 - padding
							} else {
								x = existingRight + width/2 + padding
							}
							// 重置Y位置
							y = existing.labelY
							continue
						}
					}
					y = newY
				}

				// 重新计算当前标签的边界框
				top = y
				bottom = y + height
				break
			}
		}

		if !overlap {
			break
		}
	}

	return x, y
}
