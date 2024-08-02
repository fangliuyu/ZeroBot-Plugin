package base

import (
	"math/rand"
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
}

func randText(text ...string) message.MessageSegment {
	return message.Text(text[rand.Intn(len(text))])
}
func randImage(fileList ...string) message.MessageSegment {
	name := fileList[rand.Intn(len(fileList))]
	return message.Image("file://"+file.BOTPATH+"/"+engine.DataFolder()+name, name)
}
