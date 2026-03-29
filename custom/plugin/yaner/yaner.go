package base

import (
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/process"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/RomiChan/syncx"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension/rate"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var (
	sm   syncx.Map[int64, string]
	poke = rate.NewManager[int64](time.Minute*5, 11) // 戳一戳
)

func init() {
	engine.OnFullMatch("", zero.OnlyToMe).SetBlock(true).Limit(ctxext.LimitByGroup).
		Handle(func(ctx *zero.Ctx) {
			nickname := zero.BotConfig.NickName[1]
			time.Sleep(time.Second * 1)
			ctx.SendChain(message.Text(
				[]string{
					"嗨~" + nickname + "在窥屏哦",
					"我在听",
					"请问找" + nickname + "有什么事吗",
					"？怎么了",
				}[rand.Intn(4)],
			))
		})
	// 戳一戳
	engine.On("notice/notify/poke", zero.OnlyToMe).SetBlock(false).Limit(ctxext.LimitByGroup).
		Handle(func(ctx *zero.Ctx) {
			if !poke.Load(ctx.Event.GroupID).AcquireN(1) {
				return // 最多戳11次
			}
			nickname := zero.BotConfig.NickName[1]
			info := ctx.GetGroupMemberInfo(ctx.Event.GroupID, ctx.Event.SelfID, true)
			ad := info.Get("role").String()
			time.Sleep(time.Second * 1)
			switch {
			case ad == "admin" && rand.Intn(11) < 3:
				ctx.SendChain(randText(
					"大坏蛋，吃"+nickname+"一拳!",
					"哼,"+nickname+"生气了！ヾ(≧へ≦)〃",
					"来自"+nickname+"对大坏蛋的反击!",
				))
				ctx.SetGroupBan(
					ctx.Event.GroupID,
					ctx.Event.UserID,      // 要禁言的人的qq
					(rand.Int63n(5)+1)*60, // 要禁言的时间
				)
			case rand.Intn(11) < 3:
				ctx.SendChain(randText(
					"大坏蛋，吃"+nickname+"一拳!",
					nickname+"生气了！ヾ(≧へ≦)〃",
					"来自"+nickname+"对大坏蛋的反击!",
				))
				ctx.Send(message.Poke(ctx.Event.UserID))
			default:
				ctx.SendChain(randText(
					"捏"+nickname+"的人是大坏蛋！",
					"吖,"+nickname+"的脸不是拿来捏的！",
					"啊!~"+nickname+"要生气了哦",
					"?",
					"请不要捏"+nickname+" >_<",
				))
			}
		})
	engine.OnKeywordGroup([]string{"好吗", "行不行", "能不能", "可不可以"}).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			process.SleepAbout1sTo2s()
			if rand.Intn(4) == 0 {
				nickname := zero.BotConfig.NickName[1]
				if rand.Intn(2) == 0 {
					ctx.SendChain(randText(
						"emmm..."+nickname+"..."+nickname+"觉得不行",
						"emmm..."+nickname+"..."+nickname+"觉得可以！"))
				} else {
					ctx.SendChain(randImage("Yes.jpg", "No.jpg"))
				}
			}
		})
	engine.On("message/group", zero.OnlyGroup).SetBlock(false).Handle(func(ctx *zero.Ctx) {
		gid := ctx.Event.GroupID
		raw := ctx.Event.Message.CQString()
		r, ok := sm.Load(gid)
		if !ok || r[3:] != raw {
			sm.Store(gid, "1: "+raw)
			return
		}
		c := int(r[0] - '0')
		if c%3 == 0 && rand.Intn(100) < 20 {
			c += 1
			ctx.SendChain(message.ParseMessageFromString(raw)...)
		}
		sm.Store(gid, strconv.Itoa(c+1)+": "+raw)
		if c == 5 && rand.Intn(100) < 40 {
			filepath := file.BOTPATH + "/" + engine.DataFolder() + "五时已到.gif"
			pic, err := os.ReadFile(filepath)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]", err))
				return
			}
			ctx.SendChain(message.ImageBytes(pic))
		}
	})
	engine.OnRegex(`^(\.|。)(r|R)\s*([1-9]\d*)?\s*(d|D)?\s*([1-9]\d*)?(\s*(.*))?$`).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		defaultDice := 100
		if ctx.State["regex_matched"].([]string)[5] != "" {
			defaultDice, _ = strconv.Atoi(ctx.State["regex_matched"].([]string)[5])
		}
		times := 1
		if ctx.State["regex_matched"].([]string)[3] != "" {
			times, _ = strconv.Atoi(ctx.State["regex_matched"].([]string)[3])
		}
		msg := make(message.Message, 0, 3+times)
		msg = append(msg, message.Reply(ctx.Event.MessageID))
		if ctx.State["regex_matched"].([]string)[7] != "" {
			msg = append(msg, message.Text("因为", ctx.State["regex_matched"].([]string)[7], "进行了\n"))
		}
		sum := 0
		for i := times; i > 0; i-- {
			dice := rand.Intn(defaultDice) + 1
			msg = append(msg, message.Text("🎲 => ", dice, diceRule(dice, defaultDice/2, defaultDice), "\n"))
			sum += dice
		}
		if times > 1 {
			msg = append(msg, message.Text("合计 = ", sum, diceRule(sum, defaultDice*times/2, defaultDice*times)))
		}
		ctx.Send(msg)
	})
}

func randText(text ...string) message.Segment {
	return message.Text(text[rand.Intn(len(text))])
}
func randImage(fileList ...string) message.Segment {
	name := fileList[rand.Intn(len(fileList))]
	filepath := file.BOTPATH + "/" + engine.DataFolder() + name
	pic, err := os.ReadFile(filepath)
	if err != nil {
		return message.Text("[ERROR]", err)
	}
	return message.ImageBytes(pic)
}

func diceRule(dice, decision, maxDice int) string {
	// 大成功值范围
	tenStrike := float64(maxDice) * 6 / 100
	// 成功值范围
	limit := float64(decision)
	// 大失败值范围
	fiasco := float64(maxDice) * 95 / 100
	// 骰子数
	piece := float64(dice)
	switch {
	case piece < tenStrike:
		return "(大成功!)"
	case piece > fiasco:
		return "(大失败!)"
	case piece <= fiasco && piece > limit:
		return "(失败)"
	case piece <= limit/2 && piece > limit/5:
		return "(困难成功)"
	case piece <= limit/5 && piece > 1:
		return "(极难成功)"
	default:
		return "(成功)"
	}
}
