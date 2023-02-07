// Package cybercat 云养猫
package cybercat

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	zbmath "github.com/FloatTech/floatbox/math"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

func init() {
	engine.OnRegex(`^喂猫((\d+)斤猫粮)?$`, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		gidStr := "group" + strconv.FormatInt(ctx.Event.GroupID, 10)
		uidStr := strconv.FormatInt(ctx.Event.UserID, 10)
		userInfo, err := catdata.find(gidStr, uidStr)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if userInfo.Name == "" {
			ctx.SendChain(message.Reply(id), message.Text("铲屎官你还没有属于你的主子喔,快去买一只吧!"))
			return
		}
		if userInfo.Food == 0 {
			ctx.SendChain(message.Reply(id), message.Text("铲屎官你已经没有猫粮了"))
			return
		}
		// 偷吃
		eat := 0
		if userInfo.Food > 0 && rand.Intn(10) == 1 {
			eat = rand.Intn(userInfo.Food)
			userInfo.Food -= eat
			userInfo.Satiety += eat
			if userInfo.Satiety > 80 {
				userInfo.Weight += (userInfo.Satiety - 80) / 3
				if userInfo.Weight > 200 {
					userInfo.Satiety = 0
					userInfo.Name = ""
					userInfo.Weight = 0
					ctx.SendChain(message.Reply(id), message.Text("由于猫猫在你不在期间暴饮暴食,", userInfo.Name, "已经撑死了..."))
					return
				}
				if userInfo.Satiety > 100 {
					userInfo.Mood += ((userInfo.Satiety-80)/3 - (userInfo.Weight-100)/10)
					userInfo.Satiety = 100
					if userInfo.Mood > 100 {
						userInfo.Mood = 100
					}
				}
			}
		}
		// 猫粮结算
		mun := 1
		if ctx.State["regex_matched"].([]string)[2] != "" {
			mun, _ = strconv.Atoi(ctx.State["regex_matched"].([]string)[2])
		}
		food := zbmath.Min(mun/2, userInfo.Mood)
		// 上次喂猫时间
		lastTime := time.Unix(userInfo.LastTime, 0)
		subtime := time.Since(lastTime).Hours()
		i, _ := strconv.Atoi(fmt.Sprintf("%1.0f", subtime))
		// 饱食度结算
		if userInfo.LastTime != 0 {
			userInfo.Satiety -= i
			if userInfo.Satiety < 0 {
				userInfo.Weight -= userInfo.Satiety * 2
				userInfo.Satiety = 0
				if userInfo.Weight < 0 {
					userInfo = catinfo{
						User: ctx.Event.UserID,
						Food: userInfo.Food,
					}
					err := catdata.insert(gidStr, userInfo)
					if err != nil {
						ctx.SendChain(message.Text("[ERROR]:", err))
						return
					}
					ctx.SendChain(message.Reply(id), message.Text("由于你长时间没有喂猫猫,", userInfo.Name, "已经饿死了..."))
					return
				}
			}
		}
		// 心情结算
		userInfo.Mood -= (i / 2)
		switch {
		case rand.Intn(100) > userInfo.Mood:
			ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "好像并没有心情吃东西"))
			return
		case rand.Intn(userInfo.Mood) < userInfo.Mood/3:
			userInfo.Satiety = food * 4
		default:
			userInfo.Satiety = food * 2

		}
		if userInfo.Satiety > 80 {
			userInfo.Weight += (userInfo.Satiety - 80) / 3
			if userInfo.Weight > 200 {
				userInfo.Satiety = 0
				userInfo.Name = ""
				userInfo.Weight = 0
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "已经撑死了..."))
				return
			}
			if userInfo.Satiety > 100 {
				userInfo.Mood += ((userInfo.Satiety-80)/3 - (userInfo.Weight-100)/10)
				userInfo.Satiety = 100
				if userInfo.Mood > 100 {
					userInfo.Mood = 100
				}
			}
		}
		err = catdata.insert(gidStr, userInfo)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		ctx.SendChain(message.Reply(id), message.Text("猫猫吃完了", userInfo.Name, "当前信息如下:\n",
			"\n品种: "+userInfo.Type, "\n饱食度: ", userInfo.Satiety, "\n心情: ", userInfo.Mood, "\n体重: ", userInfo.Weight, "\n\n你的剩余猫粮(斤): "+userInfo.Type))
	})
}
