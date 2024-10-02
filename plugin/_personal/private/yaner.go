package base

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/process"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension/rate"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var (
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
			switch {
			case rand.Intn(11) < 3:
				time.Sleep(time.Second * 1)
				ctx.SendChain(message.Poke(ctx.Event.UserID))
				ctx.SendChain(randText(
					"大坏蛋，吃"+nickname+"一拳!",
					nickname+"生气了！ヾ(≧へ≦)〃",
					"来自"+nickname+"对大坏蛋的反击!",
				))
				time.Sleep(time.Second * 2)
				if rand.Intn(100) < 50 {
					ctx.SetGroupBan(
						ctx.Event.GroupID,
						ctx.Event.UserID, // 要禁言的人的qq
						rand.Int63n(5)+1, // 要禁言的时间
					)
				}
			default:
				time.Sleep(time.Second * 1)
				ctx.SendChain(randText(
					"捏"+nickname+"的人是大坏蛋！",
					nickname+"的脸不是拿来捏的！",
					nickname+"要生气了哦",
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
	engine.OnRegex(`^(\.|。)(r|R)([1-9]\d*)?\s*(d|D)?\s*([1-9]\d*)?( (.*))?$`).SetBlock(true).Handle(func(ctx *zero.Ctx) {
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
			msg = append(msg, message.Text("🎲 => ", dice, diceRule(0, dice, defaultDice/2, defaultDice), "\n"))
			sum += dice
		}
		if times > 1 {
			msg = append(msg, message.Text("合计 = ", sum, diceRule(0, sum, defaultDice*times/2, defaultDice*times)))
		}
		ctx.Send(msg)
	})
}

func randText(text ...string) message.MessageSegment {
	return message.Text(text[rand.Intn(len(text))])
}
func randImage(fileList ...string) message.MessageSegment {
	name := fileList[rand.Intn(len(fileList))]
	return message.Image("file://"+file.BOTPATH+"/"+engine.DataFolder()+name, name)
}

func diceRule(ruleType, dice, decision, maxDice int) string {
	// 50的位置
	halflimit := float64(maxDice) / 2
	// 大成功值范围
	tenStrike := float64(maxDice) * 6 / 100
	// 成功值范围
	limit := float64(decision)
	// 大失败值范围
	fiasco := float64(maxDice) * 95 / 100
	// 骰子数
	piece := float64(dice)
	switch ruleType {
	case 1:
		switch {
		case (piece == 1 && limit < halflimit) || (limit >= halflimit && piece < tenStrike):
			return "(大成功!)"
		case (piece > fiasco && limit < halflimit) || (limit >= halflimit && dice == maxDice):
			return "(大失败!)"
		case ((piece <= fiasco && limit < halflimit) || dice != maxDice) && piece > limit:
			return "(失败)"
		case piece <= limit && piece > limit/2:
			return "(成功)"
		case piece <= limit/2 && piece > limit/5:
			return "(困难成功)"
		case piece <= limit/5 && piece > 1:
			return "(极难成功)"
		default:
			return ""
		}
	case 2:
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
	default:
		switch {
		case piece == 1:
			return "(大成功!)"
		case (piece > fiasco && limit < halflimit) || (limit >= halflimit && dice == maxDice):
			return "(大失败!)"
		case ((piece <= fiasco && limit < halflimit) || dice != maxDice) && piece > limit:
			return "(失败)"
		case piece <= limit && piece > limit/2:
			return "(成功)"
		case piece <= limit/2 && piece > limit/5:
			return "(困难成功)"
		case piece <= limit/5 && piece > 1:
			return "(极难成功)"
		default:
			return ""
		}
	}
}
