package ygoscore

import (
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	ctrl "github.com/FloatTech/zbpctrl"
	control "github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var (
	ygorules = []string{
		"一，村规：",
		"1.对方回合主要阶段最多发一次打断（包括手坑），进入战阶之后发什么都可以。",
		"2.禁止一次到位的打断（大宇宙，魔封，滑板，虚无等，鹰身女妖的吹雪，古遗物死镰等只能自己回合使用）",
		"3.禁止OTK，FTK",
		"\n二，比赛规则：",
		"1.参赛卡组要发出来让大家都看一下，然后投票选出是否可以参赛",
		"2.其他规则遵循比赛内容和本群村规",
		"\n三，暗黑决斗：",
		"1.双方指定对方一张卡，以灵魂作为赌约，进行三局两胜制决斗。",
		"2.输的一方将自己的灵魂封印到对方指定的卡，以后与对方决斗时禁止使用被封印的卡。",
	}
	ygorule = strings.Join(ygorules, "\n")
	zoomr   = []string{
		"好耶，我来学习牌技！快来这个房间吧ヾ(≧▽≦*)o",
		"打牌！房间已经给你们开好了哦~",
		"运气也是一种实力！来房间进行闪光抽卡吧！决斗者",
	}
	zooms = []string{
		"为所欲为",
		"WRGP",
		"阿克西斯",
	}
	lastzoom     = make(map[string]int64, 60)
	lastzoomtime time.Time
)

func init() {
	engine := control.Register("ygotools", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:           "游戏王开房工具",
		Help: "创建房间名：房间、单打、双打、比赛\n例: \\房间 时间=5 T卡=1 抽卡=2 起手=40\n" +
			"---可选以下参数----\n" +
			"时间=0~999  (单位:分钟)\n血量=0~99999\nT卡=(0:可使用T独,1：仅可以使用T卡)\n" +
			"抽卡=0~35(每回合抽)\n起手=1~40\n大师=(1|2|3|新大师|2020)\n" +
			"卡组=不(检查|洗切)\n卡表=卡表位号（0表示无禁卡）",
	})

	// 软件
	engine.OnFullMatchGroup([]string{"/软件", ".软件"}).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		ctx.SendChain(
			message.Text("下载链接：https://ygo233.com/download"))
	})
	// 先行卡
	engine.OnFullMatchGroup([]string{"/先行卡", ".先行卡", "先行卡"}).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		ctx.SendChain(message.Text("先行卡链接：https://ygo233.com/pre"))
	})
	// 游戏王美图
	engine.OnFullMatch("hso。").SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		ctx.SendChain(message.Image("https://img.moehu.org/pic.php?id=yu-gi-oh").Add("cache", 0))
	})
	// 村规
	engine.OnFullMatchGroup([]string{"/村规", ".村规", "村规", "群规", "暗黑决斗"}, func(ctx *zero.Ctx) bool {
		if ctx.Event.GroupID != 979031435 {
			return false
		}
		return true
	}).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		ctx.SendChain(message.Text(ygorule))
	})
	// 房间
	engine.OnRegex(`^[(.|。|\/|\\|老|开)](房间|单打|双打|比赛)(\s.*)?`).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		roomType := ctx.State["regex_matched"].([]string)[1]
		roomname := ""
		switch roomType {
		case "双打":
			roomname += "T"
		case "比赛":
			roomname += "M"
		}
		roomseting := ctx.State["regex_matched"].([]string)[2]
		regexp_rule := regexp.MustCompile(`(时间|T卡|起手|抽卡|大师|卡组|卡表|血量)=(\S+)`)
		roomregxp := regexp_rule.FindAllStringSubmatch(roomseting, -1)
		for _, roomrule := range roomregxp {
			setname := roomrule[1]
			setinfo := roomrule[2]
			if roomname != "" {
				roomname += ","
			}
			switch setname {
			case "时间":
				if "0" <= setinfo && setinfo <= "999" {
					roomname += ("TM" + setinfo)
				}
			case "T卡":
				if setinfo == "0" {
					roomname += "OT"
				} else if setinfo == "1" {
					roomname += "TO"
				}
			case "起手":
				if "1" <= setinfo && setinfo <= "40" {
					roomname += ("ST" + setinfo)
				}
			case "抽卡":
				if "0" <= setinfo && setinfo <= "35" {
					roomname += ("DR" + setinfo)
				}
			case "大师":
				if setinfo == "新大师" {
					roomname += "MR4"
				} else if setinfo == "2020" {
					roomname += "MR5"
				} else if "0" < setinfo && setinfo < "4" {
					roomname += ("MR" + setinfo)
				}
			case "卡组":
				if setinfo == "不检查" {
					roomname += "NC"
				} else if setinfo == "不洗切" {
					roomname += "NS"
				} else {
					roomname += "NC,NS"
				}
			case "卡表":
				if setinfo == "0" {
					roomname += "NF"
				} else if "0" < setinfo && setinfo < "9" {
					roomname += ("LF" + setinfo)
				}
			case "血量":
				if "0" < setinfo && setinfo <= "99999" {
					roomname += ("LP" + setinfo)
				}
			}
		}
		if !strings.Contains(roomname, "TM") && ctx.Event.GroupID == 979031435 {
			if roomname != "" {
				roomname += ","
			}
			roomname += "TM0"
		}
		if roomname != "" {
			roomname += "#"
		}
		namelen := 20 - len(roomname)
		if namelen > 6 {
			namelen = 6
		} else if namelen < 1 {
			ctx.SendChain(message.Text("房间名只支持20个字符，请减少房间规则"))
			return
		}
		now := time.Now()
		if now.Sub(lastzoomtime).Minutes() > 30 {
			lastzoomtime = now
			lastzoom = map[string]int64{}
		}
		if ctx.Event.GroupID != 979031435 {
			randname := checkroom(ctx.Event.GroupID, namelen)
			roomname += randname
		} else {
			randname := checkroom(ctx.Event.GroupID, 0)
			roomname += randname
		}
		ctx.SendChain(message.Text(zoomr[rand.Intn(len(zoomr))]))
		ctx.SendChain(message.Text(roomname))
	})
}

// 判断房间名
func checkroom(gid int64, rommlen int) (randname string) {
	now := time.Now()
	if lastzoom != nil {
		for randname, groupid := range lastzoom {
			if groupid == gid && now.Sub(lastzoomtime).Seconds() < 20 {
				return randname
			} else if groupid == gid {
				break
			}
		}
	}
	switch rommlen {
	case 0:
		randname = zooms[rand.Intn(len(zooms))]
	default:
		randname = RandString(rommlen)
	}
	lastzoom[randname] = gid
	return
}

// 生成token值
func RandString(length int) string {
	rand.Seed(time.Now().UnixNano())
	rs := make([]string, length)
	for start := 0; start < length; start++ {
		t := rand.Intn(10)
		if start < 4 {
			rs = append(rs, string(rune(rand.Int63n(20901)+19968)))
		} else if t < 3 {
			rs = append(rs, strconv.Itoa(rand.Intn(10)))
		} else if t < 6 {
			rs = append(rs, string(rune(rand.Intn(26)+65)))
		} else if t < 9 {
			rs = append(rs, string(rune(rand.Intn(26)+97)))
		} else {
			rs = append(rs, string(rune(rand.Int63n(20901)+19968)))
		}
	}
	return strings.Join(rs, "")
}

func randText(text ...string) message.MessageSegment {
	return message.Text(text[rand.Intn(len(text))])
}
