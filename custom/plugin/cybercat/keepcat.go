// Package cybercat 云养猫
package cybercat

import (
	"math"
	"math/rand"
	"strconv"
	"time"

	zbmath "github.com/FloatTech/floatbox/math"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

func init() {
	engine.OnFullMatch("猫猫状态", zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		gidStr := "group" + strconv.FormatInt(ctx.Event.GroupID, 10)
		uidStr := strconv.FormatInt(ctx.Event.UserID, 10)
		userInfo, err := getNewCatData(gidStr, uidStr)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if userInfo == (&catInfo{}) || userInfo.Name == "" {
			ctx.SendChain(message.Reply(id), message.Text("铲屎官你还没有属于你的主子喔,快去买一只吧!"))
			return
		}
		if userInfo.Weight <= 0 {
			userInfo.Weight = 2
			if userInfo.SubTime > 72 {
				err = catdata.catDie(gidStr, uidStr)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "由于瘦骨如柴,已经难以存活去世了..."))
				return
			}
		}
		/**************************获取工作状态*************************************/
		stauts := "休闲中"
		if userInfo.Work != 0 {
			subtime := time.Since(time.Unix(userInfo.WorkTime, 0)).Hours()
			if subtime >= userInfo.Work {
				userInfo.WorkTime = 0
				userInfo.Work = 0
				exp := int(subtime) * 5
				userInfo.Experience += exp
				stauts = "修炼结束了,进化值增加" + strconv.Itoa(exp)
				err = catdata.updateCatInfo(gidStr, userInfo)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
			} else {
				overwork := time.Unix(userInfo.WorkTime, 0).Add(time.Hour * time.Duration(userInfo.Work))
				stauts = overwork.Format("修炼中\n(将在01月02日15:04出关)")
			}
		}
		/***************************************************************/
		if userInfo.SubTime > 14 {
			stauts += "\n受饿中"
		}
		/***************************************************************/
		switch {
		case userInfo.Mood <= 0 && rand.Intn(100) < 10:
			if err = catdata.catDie(gidStr, uidStr); err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "和你的感情淡了,选择了离家出走"))
			return
		case userInfo.Weight >= 25:
			if 100*rand.Float64() > 60 {
				if err = catdata.catDie(gidStr, uidStr); err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "由于太胖了,已经难以存活去世了..."))
				return
			}
		}
		if userInfo.Mood < 0 {
			userInfo.Mood = 0
		} else if userInfo.Mood > 100 {
			userInfo.Mood = 100
		}
		err = catdata.updateCatInfo(gidStr, userInfo)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if userInfo.Experience > 2000 {
			if rand.Intn(100) < 40 {
				userInfo.Type = "猫娘"
				userInfo.Breed += 1
				userInfo.Weight = 2 + rand.Float64()*1
				userInfo.Experience -= 1000
				userInfo.LastTime = time.Now().Unix()
				if err = catdata.updateCatInfo(gidStr, userInfo); err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "进化值圆满，顿悟成功，进化成猫娘了!\n可以发送“上传猫猫照片”修改图像了喔"))

			} else if rand.Intn(100) < 40 {
				if rand.Intn(100) < 40 {
					userInfo.Weight = 2 + rand.Float64()*1
					userInfo.Experience = rand.Intn(500)
					userInfo.LastTime = time.Now().Unix()
					if err = catdata.updateCatInfo(gidStr, userInfo); err != nil {
						ctx.SendChain(message.Text("[ERROR]:", err))
						return
					}
					ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "进化值过于饱满，身体承受不了爆体受伤，被压制在了原形态"))
				} else {
					err = catdata.catDie(gidStr, uidStr)
					if err != nil {
						ctx.SendChain(message.Text("[ERROR]:", err))
						return
					}
					ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "进化值过于饱满，身体承受不了爆体而亡"))
					return
				}
			}
		}
		/***************************************************************/
		avatarResult, err := userInfo.avatar(ctx.Event.GroupID)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		foodinfo, err := catdata.getHomeInfo(uidStr)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]: got food info fail.", err))
		}
		text := "品种: " + userInfo.Type
		if userInfo.Type == "猫娘" && userInfo.Breed > 1 {
			text += strconv.Itoa(userInfo.Breed) + "世"
		}
		ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "当前信息如下:\n"),
			message.ImageBytes(avatarResult),
			message.Text(text,
				"\n饱食度: ", strconv.FormatFloat(userInfo.Satiety, 'f', 0, 64),
				"\n心情: ", userInfo.Mood,
				"\n体重: ", strconv.FormatFloat(userInfo.Weight, 'f', 2, 64),
				"\n进化值: ", userInfo.Experience, "/1000",
				"\n状态:\n", stauts,
				"\n\n你的剩余猫粮(斤): ", strconv.FormatFloat(foodinfo.Food, 'f', 2, 64)))
	})
	engine.OnRegex(`^喂猫((\d+(.\d+)?)斤猫粮)?$`, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		gidStr := "group" + strconv.FormatInt(ctx.Event.GroupID, 10)
		uidStr := strconv.FormatInt(ctx.Event.UserID, 10)
		userInfo, err := getNewCatData(gidStr, uidStr)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if userInfo == (&catInfo{}) || userInfo.Name == "" {
			ctx.SendChain(message.Reply(id), message.Text("铲屎官你还没有属于你的主子喔,快去买一只吧!"))
			return
		}
		if userInfo.Weight <= 0 {
			userInfo.Weight = 2
			if userInfo.SubTime > 72 {
				err = catdata.catDie(gidStr, uidStr)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "由于瘦骨如柴,已经难以存活去世了..."))
				return
			}
		}
		foodinfo, err := catdata.getHomeInfo(uidStr)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		/***************************************************************/
		stauts := "休闲中"
		if userInfo.Work != 0 {
			subtime := time.Since(time.Unix(userInfo.WorkTime, 0)).Hours()
			if subtime >= userInfo.Work {
				userInfo.WorkTime = 0
				userInfo.Work = 0
				exp := int(subtime) * 5
				userInfo.Experience += exp
				stauts = "修炼结束了,进化值增加" + strconv.Itoa(exp)
				err = catdata.updateCatInfo(gidStr, userInfo)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
			} else {
				overwork := time.Unix(userInfo.WorkTime, 0).Add(time.Hour * time.Duration(userInfo.Work))
				stauts = overwork.Format("修炼中\n(将在01月02日15:04出关)")
			}
		}
		/***************************************************************/
		food := 0.0
		needFood := (100 - userInfo.Satiety) * userInfo.Weight / 150
		if ctx.State["regex_matched"].([]string)[2] != "" {
			food, _ = strconv.ParseFloat(ctx.State["regex_matched"].([]string)[2], 64)
			if food > needFood && rand.Intn(100) > 50 {
				food = math.Min(needFood*1.5, food)
			}
		} else {
			food = math.Min(needFood*1.1, foodinfo.Food)
		}
		if foodinfo.Food <= 0 || foodinfo.Food < food {
			food = foodinfo.Food
			stauts += "\n没有足够的猫粮了"
		} else {
			stauts += "\n刚刚的食物很美味"
		}
		switch {
		case userInfo.Satiety >= 100 && rand.Intn(100) < 10:
			food = 0
			stauts += "\n" + userInfo.Name + "拍了拍肚子，表示太饱了!"
		case userInfo.Satiety > 80 && rand.Intn(100) < 60:
			food = needFood
			stauts += "\n食物实在太多了!"
		case userInfo.Weight > 10.0 && food < 1:
			ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "觉得食物太少，骂骂咧咧的叫着"))
			return
		}
		foodinfo.Food -= food
		userInfo.Mood += 80 + int(food)*20
		userInfo.Satiety += food * 250.0 / userInfo.Weight
		userInfo.SubTime = 0
		/****************************结算食物***********************************/
		if userInfo.Satiety > 100 {
			userInfo.Weight += (userInfo.Satiety - 90) / 100
			userInfo.Satiety = 100
		}
		if userInfo.Mood > 100 {
			exp := (userInfo.Mood - 100) / 5
			userInfo.Experience += exp
			userInfo.Mood = 100
			stauts += "\n心情愉悦，获得感悟，进化值+" + strconv.Itoa(exp)
		} else if userInfo.Mood < 0 {
			userInfo.Mood = 0
		}
		err = catdata.updateCatInfo(gidStr, userInfo)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		/****************************空闲时间猫体力的减少计算***********************************/
		switch {
		case userInfo.Mood <= 0 && rand.Intn(100) < 10:
			if err = catdata.catDie(gidStr, uidStr); err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "和你的感情淡了,选择了离家出走"))
			return
		case userInfo.Weight >= 25:
			if 100*rand.Float64() > 60 {
				if err = catdata.catDie(gidStr, uidStr); err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "由于太胖了,已经难以存活去世了..."))
				return
			}
		}
		if userInfo.Experience > 2000 {
			if rand.Intn(100) < 40 {
				userInfo.Type = "猫娘"
				userInfo.Breed += 1
				userInfo.Weight = 2 + rand.Float64()*10
				userInfo.Experience -= 1000
				userInfo.LastTime = time.Now().Unix()
				if err = catdata.updateCatInfo(gidStr, userInfo); err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "进化值圆满，顿悟成功，进化成猫娘了!\n可以发送“上传猫猫照片”修改图像了喔"))

			} else if rand.Intn(100) < 40 {
				if rand.Intn(100) < 40 {
					userInfo.Weight = 2 + rand.Float64()*10
					userInfo.Experience = rand.Intn(500)
					userInfo.LastTime = time.Now().Unix()
					if err = catdata.updateCatInfo(gidStr, userInfo); err != nil {
						ctx.SendChain(message.Text("[ERROR]:", err))
						return
					}
					ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "进化值过于饱满，身体承受不了爆体受伤，被压制在了原形态"))
				} else {
					err = catdata.catDie(gidStr, uidStr)
					if err != nil {
						ctx.SendChain(message.Text("[ERROR]:", err))
						return
					}
					ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "进化值过于饱满，身体承受不了爆体而亡"))
					return
				}
			}
		}
		if err = catdata.updateHomeInfo(&foodinfo); err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		/****************************保存数据***********************************/
		avatarResult, err := userInfo.avatar(ctx.Event.GroupID)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		text := "品种: " + userInfo.Type
		if userInfo.Type == "猫娘" && userInfo.Breed > 1 {
			text += strconv.Itoa(userInfo.Breed) + "世"
		}
		ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "当前信息如下:\n"),
			message.ImageBytes(avatarResult),
			message.Text(text,
				"\n饱食度: ", strconv.FormatFloat(userInfo.Satiety, 'f', 0, 64),
				"\n心情: ", userInfo.Mood,
				"\n体重: ", strconv.FormatFloat(userInfo.Weight, 'f', 2, 64),
				"\n进化值: ", userInfo.Experience, "/1000",
				"\n状态:\n", stauts,
				"\n\n你的剩余猫粮(斤): ", strconv.FormatFloat(foodinfo.Food, 'f', 2, 64)))
	})
	engine.OnRegex(`^猫猫修炼(([1-9])小时)?$`, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		gidStr := "group" + strconv.FormatInt(ctx.Event.GroupID, 10)
		uidStr := strconv.FormatInt(ctx.Event.UserID, 10)
		userInfo, err := getNewCatData(gidStr, uidStr)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if userInfo == (&catInfo{}) || userInfo.Name == "" {
			ctx.SendChain(message.Reply(id), message.Text("铲屎官你还没有属于你的主子喔,快去买一只吧!"))
			return
		}
		if userInfo.Weight <= 0 {
			userInfo.Weight = 2
			if userInfo.SubTime > 72 {
				err = catdata.catDie(gidStr, uidStr)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "由于瘦骨如柴,已经难以存活去世了..."))
				return
			}
		}
		/***************************************************************/
		if userInfo.Work != 0 {
			subtime := time.Since(time.Unix(userInfo.WorkTime, 0)).Hours()
			if subtime >= userInfo.Work {
				userInfo.WorkTime = 0
				userInfo.Work = 0
				exp := int(subtime) * 5
				userInfo.Experience += exp
				err = catdata.updateCatInfo(gidStr, userInfo)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
			} else {
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "已经在努力的修炼了"))
				return
			}
		}
		if userInfo.Satiety > 90 && rand.Intn(100) > zbmath.Max(userInfo.Mood*2-userInfo.Mood/2, 50) {
			ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "好像并没有心情去修炼"))
			return
		}
		/***************************************************************/
		workTime := 9.0
		if ctx.State["regex_matched"].([]string)[2] != "" {
			workTime, _ = strconv.ParseFloat(ctx.State["regex_matched"].([]string)[2], 64)
		}
		userInfo.WorkTime = time.Now().Unix()
		userInfo.Work = workTime
		if err = catdata.updateCatInfo(gidStr, userInfo); err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "开始修炼了"))
	})
	engine.OnFullMatch("猫猫取消修炼", zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		gidStr := "group" + strconv.FormatInt(ctx.Event.GroupID, 10)
		uidStr := strconv.FormatInt(ctx.Event.UserID, 10)
		userInfo, err := getNewCatData(gidStr, uidStr)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if userInfo == (&catInfo{}) || userInfo.Name == "" {
			ctx.SendChain(message.Reply(id), message.Text("铲屎官你还没有属于你的主子喔,快去买一只吧!"))
			return
		}
		if userInfo.Weight <= 0 {
			userInfo.Weight = 2
			if userInfo.SubTime > 72 {
				err = catdata.catDie(gidStr, uidStr)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "由于瘦骨如柴,已经难以存活去世了..."))
				return
			}
		}
		/***************************************************************/
		if userInfo.Work != 0 {
			subtime := time.Since(time.Unix(userInfo.WorkTime, 0)).Hours()
			userInfo.WorkTime = 0
			userInfo.Work = 0
			exp := int(subtime) * 5
			userInfo.Experience += exp
			err = catdata.updateCatInfo(gidStr, userInfo)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "停止了修炼。期间获得修炼感悟已转为进化值，进化值+", strconv.Itoa(exp)))
		} else {
			ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "现在并没有在修炼"))
			return
		}
		/***************************************************************/
		if userInfo.Experience > 2000 {
			stauts := "突破成功"
			if rand.Intn(100) < 40 {
				userInfo.Type = "猫娘"
				userInfo.Breed += 1
				userInfo.Weight = 2 + rand.Float64()*10
				userInfo.Experience -= 1000
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "进化值圆满，顿悟成功，进化成猫娘了!\n可以发送“上传猫猫照片”修改图像了喔"))
			} else if rand.Intn(100) < 40 {
				if rand.Intn(100) < 40 {
					stauts = "突破失败"
					ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "进化值过于饱满，身体承受不了爆体受伤，被压制在了原形态"))
					userInfo.Weight = 2 + rand.Float64()*10
					userInfo.Experience = rand.Intn(500)
				} else {
					err = catdata.catDie(gidStr, uidStr)
					if err != nil {
						ctx.SendChain(message.Text("[ERROR]:", err))
						return
					}
					ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "进化值过于饱满，身体承受不了爆体而亡"))
					return
				}
			}
			userInfo.LastTime = time.Now().Unix()
			if err = catdata.updateCatInfo(gidStr, userInfo); err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			/***************************************************************/
			avatarResult, err := userInfo.avatar(ctx.Event.GroupID)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			foodinfo, _ := catdata.getHomeInfo(uidStr)
			text := "品种: " + userInfo.Type
			if userInfo.Type == "猫娘" && userInfo.Breed > 1 {
				text += strconv.Itoa(userInfo.Breed) + "世"
			}
			ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "当前信息如下:\n"),
				message.ImageBytes(avatarResult),
				message.Text(text,
					"\n饱食度: ", strconv.FormatFloat(userInfo.Satiety, 'f', 0, 64),
					"\n心情: ", userInfo.Mood,
					"\n体重: ", strconv.FormatFloat(userInfo.Weight, 'f', 2, 64),
					"\n状态:\n", stauts,
					"\n\n你的剩余猫粮(斤): ", strconv.FormatFloat(foodinfo.Food, 'f', 2, 64)))
		}
	})
	engine.OnFullMatchGroup([]string{"逗猫", "撸猫", "rua猫", "mua猫", "玩猫", "摸猫"}, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		gidStr := "group" + strconv.FormatInt(ctx.Event.GroupID, 10)
		uidStr := strconv.FormatInt(ctx.Event.UserID, 10)
		userInfo, err := getNewCatData(gidStr, uidStr)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if userInfo == (&catInfo{}) || userInfo.Name == "" {
			ctx.SendChain(message.Reply(id), message.Text("铲屎官你还没有属于你的主子喔,快去买一只吧!"))
			return
		}
		if userInfo.Weight <= 0 {
			userInfo.Weight = 2
			if userInfo.SubTime > 72 {
				err = catdata.catDie(gidStr, uidStr)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "由于瘦骨如柴,已经难以存活去世了..."))
				return
			}
		}
		/***************************************************************/
		if userInfo.Work != 0 {
			subtime := time.Since(time.Unix(userInfo.WorkTime, 0)).Hours()
			if subtime >= userInfo.Work {
				userInfo.WorkTime = 0
				userInfo.Work = 0
				exp := int(subtime) * 5
				userInfo.Experience += exp
				err = catdata.updateCatInfo(gidStr, userInfo)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
			}
		}
		/***************************************************************/
		thingInfo, err := catdata.getHomeInfo(uidStr)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		thingInfo.User = ctx.Event.UserID
		if time.Unix(thingInfo.LastTime, 0).Day() != time.Now().Day() {
			thingInfo.LastTime = time.Now().Unix()
			thingInfo.Rua = 0
		}
		reopText := ""
		text := ""
		upNumber := 0
		if thingInfo.Rrop1 > 0 {
			upNumber = 30
			thingInfo.Rrop1--
			reopText += "(自动使用了1逗猫棒)"
		}
		choose := rand.Intn(100) + rand.Intn(100-userInfo.Mood) + upNumber - thingInfo.Rua*2
		if choose < 50 {
			userInfo.Mood -= rand.Intn(zbmath.Max(1, userInfo.Mood-upNumber))
			text += "不耐烦的走掉了,心情降低至"
			if choose < 1 {
				choose = 1
			}
		} else {
			userInfo.Mood += 20 + rand.Intn(100-userInfo.Mood)
			text += "被调教得屁股高跷呢!心情提高至"
			if choose > 100 {
				choose = 100
			}
		}
		addtest := ""
		if userInfo.Mood < 0 {
			userInfo.Mood = 0
		} else if userInfo.Mood > 100 {
			exp := 5 + rand.Intn(10)
			userInfo.Experience += exp
			userInfo.Mood = 100
			addtest = "。心情愉悦，获得感悟，进化值+" + strconv.Itoa(exp)
		}
		thingInfo.Rua++
		err = catdata.updateHomeInfo(&thingInfo)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if userInfo.Mood < 50 && userInfo.Work != 0 {
			userInfo.Work = 0
			userInfo.WorkTime = 0
			err = catdata.updateCatInfo(gidStr, userInfo)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			addtest = "。心情过低,决定停止了修炼"
		}
		/***************************************************************/
		if err = catdata.updateCatInfo(gidStr, userInfo); err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		ctx.SendChain(message.Reply(id), message.Text("(🎲rd100=>", 101-choose, ")\n", userInfo.Name, text, userInfo.Mood, reopText, addtest))
	})
	engine.OnFullMatch("猫猫突破", getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		gidStr := "group" + strconv.FormatInt(ctx.Event.GroupID, 10)
		uidStr := strconv.FormatInt(ctx.Event.UserID, 10)
		userInfo, err := getNewCatData(gidStr, uidStr)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if userInfo == (&catInfo{}) || userInfo.Name == "" {
			ctx.SendChain(message.Reply(id), message.Text("铲屎官你还没有属于你的主子喔,快去买一只吧!"))
			return
		}
		if userInfo.Weight <= 0 {
			userInfo.Weight = 2
			if userInfo.SubTime > 72 {
				err = catdata.catDie(gidStr, uidStr)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "由于瘦骨如柴,已经难以存活去世了..."))
				return
			}
		}
		/**************************获取工作状态*************************************/
		if userInfo.Work != 0 {
			subtime := time.Since(time.Unix(userInfo.WorkTime, 0)).Hours()
			if subtime >= userInfo.Work {
				userInfo.WorkTime = 0
				userInfo.Work = 0
				exp := int(subtime) * 5
				userInfo.Experience += exp
				err = catdata.updateCatInfo(gidStr, userInfo)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
			}
		}
		/***************************************************************/
		if userInfo.Experience > 1000 {
			stauts := "突破成功"
			if rand.Intn(100) < 40 {
				userInfo.Type = "猫娘"
				userInfo.Breed += 1
				userInfo.Weight = 2 + rand.Float64()
				userInfo.Experience -= 1000
				userInfo.LastTime = time.Now().Unix()
				if err = catdata.updateCatInfo(gidStr, userInfo); err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "进化值圆满，顿悟成功，进化成猫娘了!\n可以发送“上传猫猫照片”修改图像了喔"))
			} else if rand.Intn(100) < 40 {
				if rand.Intn(100) < 40 {
					stauts = "突破失败"
					userInfo.Weight = rand.Float64() * userInfo.Weight
					userInfo.Experience = rand.Intn(500)
					userInfo.LastTime = time.Now().Unix()
					if err = catdata.updateCatInfo(gidStr, userInfo); err != nil {
						ctx.SendChain(message.Text("[ERROR]:", err))
						return
					}
					ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "进化值过于饱满，身体承受不了爆体受伤，被压制在了原形态"))
				} else {
					err = catdata.catDie(gidStr, uidStr)
					if err != nil {
						ctx.SendChain(message.Text("[ERROR]:", err))
						return
					}
					ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "进化值过于饱满，身体承受不了爆体而亡"))
					return
				}
			}
			/***************************************************************/
			avatarResult, err := userInfo.avatar(ctx.Event.GroupID)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			foodinfo, _ := catdata.getHomeInfo(uidStr)
			text := "品种: " + userInfo.Type
			if userInfo.Type == "猫娘" && userInfo.Breed > 1 {
				text += strconv.Itoa(userInfo.Breed) + "世"
			}
			ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "当前信息如下:\n"),
				message.ImageBytes(avatarResult),
				message.Text(text,
					"\n饱食度: ", strconv.FormatFloat(userInfo.Satiety, 'f', 0, 64),
					"\n心情: ", userInfo.Mood,
					"\n体重: ", strconv.FormatFloat(userInfo.Weight, 'f', 2, 64),
					"\n状态:\n", stauts,
					"\n\n你的剩余猫粮(斤): ", strconv.FormatFloat(foodinfo.Food, 'f', 2, 64)))
		} else {
			ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "修为尚未饱满，无法突破"))
			return
		}
	})
}
