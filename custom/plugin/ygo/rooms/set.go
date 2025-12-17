// Package ygo 一些关于ygo的插件
package ygo

import (
	"math/rand"
	"strconv"
	"strings"

	zbmath "github.com/FloatTech/floatbox/math"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var (
	ygorules = []string{
		"一,村规:",
		"1.对方回合主要阶段最多发一次打断（包括手坑）,进入战阶之后发什么都可以。",
		"2.禁止一次到位的打断（大宇宙,魔封,滑板,虚无等,鹰身女妖的吹雪,古遗物死镰等只能自己回合使用）",
		"3.禁止OTK,FTK,削手",
		"\n二,比赛规则:",
		"1.参赛卡组要发出来让大家都看一下,然后投票选出是否可以参赛",
		"2.其他规则遵循比赛内容和本群村规",
		"\n三,暗黑决斗:",
		"1.双方指定对方一张卡,以灵魂作为赌约,进行三局两胜制决斗。",
		"2.输的一方将自己的灵魂封印到对方指定的卡,以后与对方决斗时禁止使用被封印的卡。",
	}
	ygorule = strings.Join(ygorules, "\n")
	zoomr   = []string{
		"好耶,我来学习牌技！快来这个房间吧ヾ(≧▽≦*)o",
		"打牌！房间已经给你们开好了哦~",
		"运气也是一种实力！来房间进行闪光抽卡吧！决斗者",
	}
	zooms = []string{
		"为所欲为",
		"WRGP",
		"阿克西斯",
	}
)

// ShellRule Example
// 本插件仅作为演示
// Note: 只有带 flag 的Tag的字段才会注册,
// 支支持 bool, int, string, float64 四种类型
type RoomRule struct {
	T      int    `flag:"tm"` // 0~99  (每回合时间，单位:分钟)
	TM     int    `flag:"时间"` // 0~99  (每回合时间，单位:分钟)
	L      int    `flag:"lp"` // 0~99999
	LP     int    `flag:"血"`  // 0~99999
	Dr     int    `flag:"dr"` // 0~35  (每回合抽卡数)
	Draw   int    `flag:"抽"`  // 0~35  (每回合抽卡数)
	H      int    `flag:"st"` // 1~40  (起手抽卡数)
	Hand   int    `flag:"起"`  // 1~40  (起手抽卡数)
	M      string `flag:"mr"` // 1|2|3|新大师|2020
	Master string `flag:"大师"` // 1|2|3|新大师|2020
	R      int    `flag:"lf"` // 卡表位号  (0表示无禁卡)
	Rule   int    `flag:"卡表"` // 卡表位号  (0表示无禁卡)
	Match  bool   `flag:"m"`  // 开启BO3房
	Doubel bool   `flag:"t"`  // 开启双打房
	OT     bool   `flag:"ot"` // 可使用T独, OT混合卡池
	C      bool   `flag:"nc"` // 不检查卡组
	F      bool   `flag:"ns"` // 不洗切卡组
}

func init() {
	// 村规
	engine.OnFullMatchGroup([]string{"/村规", ".村规", "村规", "群规", "暗黑决斗"}, func(ctx *zero.Ctx) bool {
		return ctx.Event.GroupID == 979031435
	}).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		ctx.SendChain(message.Text(ygorule))
	})
	// 房间
	engine.OnPrefix("/记录房间").Handle(func(ctx *zero.Ctx) {
		roomName := strings.TrimSpace(ctx.State["args"].(string))
		roomData := playerDB{
			ID:   ctx.Event.UserID,
			Room: roomName,
		}
		if err := database.update(roomTable, &roomData); err != nil {
			ctx.SendChain(message.Text("[ygorooms] ERROR: ", err, "\nEXP: 记录房间失败"))
			return
		}
		ctx.SendChain(message.Text("房间记录成功"))
	})
	engine.OnShell("/房间", RoomRule{}).Handle(func(ctx *zero.Ctx) {
		rule := ctx.State["flag"].(*RoomRule) // Note: 指针类型
		data, err := database.find(roomTable, ctx.Event.UserID)
		if err != nil {
			ctx.SendChain(message.Text("[ygorooms] ERROR: ", err, "\nEXP: 获取房间失败, 将随机生成房间"))
		}
		roomData := data.(playerDB)
		name := ""
		if roomData.Room != "" {
			name = roomData.Room
		}
		if name == "" {
			for _, v := range ctx.State["args"].([]string) {
				if name != "" {
					name += "的"
				}
				name += v
			}
		}
		if name == "" {
			name = zooms[rand.Intn(len(zooms))]
		}
		roomname := []string{}
		if rule.Doubel {
			roomname = append(roomname, "T")
		}
		if rule.Match {
			roomname = append(roomname, "M")
		}
		if (rule.TM >= 0 || rule.T >= 0) && (zbmath.Max(rule.TM, rule.T) <= 99) {
			timeSet := zbmath.Max(rule.TM, rule.T)
			roomname = append(roomname, "TM"+strconv.Itoa(timeSet))
		}
		if (rule.LP > 0 || rule.L > 0) && (zbmath.Max(rule.LP, rule.L) <= 99999) {
			lpSet := zbmath.Max(rule.LP, rule.L)
			roomname = append(roomname, "LP"+strconv.Itoa(lpSet))
		}
		if (rule.Draw >= 0 || rule.Dr >= 0) && (zbmath.Max(rule.Draw, rule.Dr) <= 35) {
			drawSet := zbmath.Max(rule.Draw, rule.Dr)
			roomname = append(roomname, "DR"+strconv.Itoa(drawSet))
		}
		if (rule.Hand >= 0 || rule.H >= 0) && (zbmath.Max(rule.Hand, rule.H) <= 40) {
			handSet := zbmath.Max(rule.Hand, rule.H)
			roomname = append(roomname, "ST"+strconv.Itoa(handSet))
		}
		if rule.Master != "" || rule.M != "" {
			masterSet := rule.Master
			if masterSet == "" {
				masterSet = rule.M
			}
			switch masterSet {
			case "新大师":
				roomname = append(roomname, "MR4")
			case "2020":
				roomname = append(roomname, "MR5")
			case "1", "2", "3":
				roomname = append(roomname, "MR"+masterSet)
			}
		}
		if rule.Rule >= 0 || rule.R >= 0 {
			ruleSet := zbmath.Max(rule.Rule, rule.R)
			if ruleSet == 0 {
				roomname = append(roomname, "NF")
			} else {
				roomname = append(roomname, "LF"+strconv.Itoa(rule.Rule))
			}
		}
		if rule.OT {
			roomname = append(roomname, "OT")
		}
		if rule.C {
			roomname = append(roomname, "NC")
		}
		if rule.F {
			roomname = append(roomname, "NS")
		}
		finalname := strings.Join(roomname, ",")
		if finalname != "" {
			finalname += "#"
		} else {
			finalname += "OT#"
		}
		namelen := 20 - len(finalname)
		if namelen < 4 {
			ctx.SendChain(message.Text("房间名只支持20个字符,请减少房间规则"))
			return
		}
		finalname += name
		ctx.SendChain(message.Text(zoomr[rand.Intn(len(zoomr))]))
		ctx.SendChain(message.Text(finalname))
	})
}
