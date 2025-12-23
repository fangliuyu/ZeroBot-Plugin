// Package ygo 一些关于ygo的插件
package ygo

import (
	"encoding/json"
	"fmt"
	"math"
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
	myCardPieAPI    = "https://sapi.moecube.com:444/ygopro/analytics/deck/type?type=%s&source=mycard-%s"
	myCardPlayerAPI = "https://sapi.moecube.com:444/ygopro/arena/user?username=%s"
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
		url := fmt.Sprintf(myCardPlayerAPI, name)
		data, err := web.GetData(url)
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
		if i < 9 {
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
	const (
		width  = 680
		height = 600
		radius = 200
	)

	canvas := gg.NewContext(width, height)
	canvas.SetRGB(1, 1, 1) // 背景色为白色
	canvas.Clear()

	font, err := file.GetLazyData(text.BoldFontFile, control.Md5File, true)
	if err != nil {
		return nil, err
	}
	if err = canvas.ParseFontFace(font, 12); err != nil {
		return nil, err
	}

	startAngle := -math.Pi / 2 // 从顶部开始绘制
	lastLabelY := -1.0
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
		// 防止标签重叠
		if math.Abs(labelY-lastLabelY) < 15 {
			if labelY > lastLabelY {
				labelY += 15
			} else {
				labelY -= 15
			}
		}
		lastLabelY = labelY
		labelText := fmt.Sprintf("%s: %d (%.2f%%)", d.label, d.value, percentage*100)
		canvas.SetRGB(0, 0, 0) // 黑色文字
		canvas.DrawStringAnchored(labelText, labelX, labelY, 0.5, 0.5)

		startAngle += angle
	}
	if err = canvas.ParseFontFace(font, 24); err != nil {
		return nil, err
	}
	_, textH := canvas.MeasureString("M")
	canvas.DrawStringAnchored("MyCard "+typ+source+"饼图", float64(width)/2, 10+textH, 0.5, 0.5)
	canvas.DrawStringAnchored("获取时间: "+time.Now().Format("2006-01-02 15:04:05"), float64(width)/2, float64(height)-10-textH, 0.5, 0.5)
	// 生成图片
	return imgfactory.ToBytes(canvas.Image())

}
