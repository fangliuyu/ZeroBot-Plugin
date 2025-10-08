// Package cybercat 云养猫
package cybercat

import (
	"image"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/FloatTech/floatbox/file"
	zbpmath "github.com/FloatTech/floatbox/math"
	"github.com/FloatTech/imgfactory"
	"github.com/FloatTech/rendercard"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/FloatTech/zbputils/img/text"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

func init() {
	engine.OnRegex(`^(喵喵|猫猫)(PK|pk)\s*\[CQ:at,qq=(\d+).*`, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		gidStr := "group" + strconv.FormatInt(ctx.Event.GroupID, 10)
		uidStr := strconv.FormatInt(ctx.Event.UserID, 10)
		if ctx.State["regex_matched"].([]string)[3] == uidStr {
			ctx.SendChain(message.Reply(id), message.Text("猫猫歪头看着你表示咄咄怪事哦~"))
			return
		}
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
				ctx.SendChain(message.Reply(id), message.Text("猫猫", userInfo.Name, "由于瘦骨如柴,已经难以存活去世了..."))
				return
			}
		}
		lastTime := time.Unix(userInfo.ArenaTime, 0)
		if time.Since(lastTime).Hours() < 24 {
			ctx.SendChain(message.Reply(id), message.Text(userInfo.Name, "已经PK过了,让它休息休息吧"))
			return
		}
		duelStr := ctx.State["regex_matched"].([]string)[3]
		duelInfo, err := getNewCatData(gidStr, duelStr)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if duelInfo == (&catInfo{}) || duelInfo.Name == "" {
			ctx.SendChain(message.Reply(id), message.Text("他还没有属于他的猫猫,无法PK"))
			return
		}
		if duelInfo.Weight <= 0 {
			duelInfo.Weight = 2
			if duelInfo.SubTime > 72 {
				err = catdata.catDie(gidStr, duelStr)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
				ctx.SendChain(message.Reply(id), message.Text("猫猫", duelInfo.Name, "由于瘦骨如柴,已经难以存活去世了..."))
				return
			}
		}
		lastTime = time.Unix(duelInfo.ArenaTime, 0)
		if time.Since(lastTime).Hours() < 24 {
			ctx.SendChain(message.Reply(id), message.Text(ctx.CardOrNickName(duelInfo.User), "的", duelInfo.Name, "已经PK过了,让它休息休息吧"))
			return
		}
		/***************************************************************/
		ctx.SendChain(message.Text("等待对方回应。(发送“取消”撤回PK)\n请对方发送“去吧猫猫”接受PK或“拒绝”结束PK"))
		recv, cancel := zero.NewFutureEvent("message", 999, false, zero.OnlyGroup, zero.RegexRule("^(去吧猫猫|取消|拒绝)$"), zero.CheckGroup(ctx.Event.GroupID), zero.CheckUser(zbpmath.Str2Int64(duelStr), userInfo.User)).Repeat()
		defer cancel()
		approve := false
		over := time.NewTimer(60 * time.Second)
		for {
			select {
			case <-over.C:
				ctx.SendChain(message.Reply(id), message.Text("对方没回应,PK取消"))
				return
			case c := <-recv:
				over.Stop()
				switch {
				case c.Event.Message.String() == "拒绝" && c.Event.UserID == duelInfo.User:
					ctx.SendChain(message.Reply(id), message.Text("对方拒绝了你的PK"))
					return
				case c.Event.Message.String() == "取消" && c.Event.UserID == userInfo.User:
					ctx.SendChain(message.Reply(id), message.Text("你取消了PK"))
					return
				case c.Event.Message.String() == "去吧猫猫" && c.Event.UserID == duelInfo.User:
					approve = true
				}
			}
			if approve {
				break
			}
		}
		/***************************************************************/
		now := time.Now().Unix()
		winer := userInfo
		loser := duelInfo
		/***************************************************************/
		mood := false
		switch {
		case userInfo.Satiety > 50 && rand.Intn(100) > zbpmath.Max(userInfo.Mood, 80):
			mood = true
			winer = duelInfo
			loser = userInfo
		case duelInfo.Satiety > 50 && rand.Intn(100) > zbpmath.Max(duelInfo.Mood, 80):
			mood = true
		}
		if mood {
			ctx.SendChain(message.Text(loser.Name, "好像并没有心情PK\n", winer.Name, "获得了比赛胜利"))
			exp := zbpmath.Max(winer.Experience/10, rand.Intn(30))
			winer.Experience += exp
			winer.ArenaTime = now
			err = catdata.updateCatInfo(gidStr, winer)
			if err == nil {
				loser.ArenaTime = now
				err = catdata.updateCatInfo(gidStr, loser)
			}
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
			}
			return
		}
		/***************************************************************/
		winLine := math.Min(userInfo.Weight, duelInfo.Weight)
		weightLine := (userInfo.Weight + duelInfo.Weight) * rand.Float64()
		fatLine := false
		if winLine > weightLine-winLine*0.1 && winLine < weightLine+winLine*0.1 {
			fatLine = true
		}
		if fatLine {
			ctx.SendChain(message.Reply(id), message.Text(duelInfo.Name, "和", userInfo.Name, "之间并没有PK的意愿呢\nPK结束"))
			return
		}
		/***************************************************************/
		winer, loser = pkweight(userInfo, duelInfo)
		messageText := make(message.Message, 0, 3)
		if rand.Intn(2) == 0 {
			messageText = append(messageText, message.Text(
				"天啊,",
				winer.Name,
				"完美的借力打力,将",
				loser.Name,
				"打趴下了",
			))
		} else {
			messageText = append(messageText, message.Text(
				"精彩!",
				winer.Name,
				"利用了PK地形,让",
				loser.Name,
				"认输了",
			))
		}
		exp := zbpmath.Max(loser.Experience/10, rand.Intn(30))
		winer.Experience += exp
		if rand.Float64()*100 < math.Max(20, loser.Weight) {
			loser.Experience -= zbpmath.Min(10, winer.Experience/10)
			if loser.Experience < 0 {
				loser.Experience = 0
			}
			messageText = append(messageText, message.Text("\n"), message.At(loser.User),
				message.Text("\n", loser.Name, "在PK中受伤了\n在医疗中心治愈过程中进化值降低至", loser.Experience))
		}
		userInfo.ArenaTime = time.Now().Unix()
		err = catdata.updateCatInfo(gidStr, winer)
		if err == nil {
			duelInfo.ArenaTime = time.Now().Unix()
			err = catdata.updateCatInfo(gidStr, loser)
		}
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
		}
		ctx.Send(messageText)
	})
	engine.OnFullMatchGroup([]string{"猫猫排行榜", "喵喵排行榜"}, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		gidStr := "group" + strconv.FormatInt(ctx.Event.GroupID, 10)
		infoList, err := catdata.getGroupCatdata(gidStr)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if len(infoList) == 0 {
			ctx.SendChain(message.Text("没有人养猫哦"))
			return
		}

		cache := filepath.Join(engine.DataFolder(), "cache")
		if file.IsNotExist(cache) {
			err = os.MkdirAll(cache, 0755)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
		}

		imgfloder := filepath.Join(cache, strconv.FormatInt(ctx.Event.GroupID, 10))
		if file.IsNotExist(imgfloder) {
			err = os.MkdirAll(imgfloder, 0755)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
		}

		rankinfo := make([]*rendercard.RankInfo, len(infoList))
		var img image.Image
		for i, info := range infoList {
			if i > 9 {
				break
			}

			aimgfile := filepath.Join(imgfloder, strconv.FormatInt(info.User, 10)+".gif")
			if file.IsNotExist(aimgfile) {
				err = file.DownloadTo(info.Picurl, aimgfile)
				if err != nil {
					ctx.SendChain(message.Text("[ERROR]:", err))
					return
				}
			}
			f, err := os.Open(filepath.Join(file.BOTPATH, aimgfile))
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			defer f.Close()
			img, _, err = image.Decode(f)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			catType := info.Type
			if catType == "猫娘" && info.Breed > 1 {
				catType += strconv.Itoa(info.Breed) + "世"
			}
			rankinfo[i] = &rendercard.RankInfo{
				TopLeftText:    info.Name,
				BottomLeftText: "主人: " + ctx.CardOrNickName(infoList[i].User),
				RightText:      catType + "(" + strconv.Itoa(info.Experience) + "/1000)",
				Avatar:         img,
			}
		}
		fontbyte, err := file.GetLazyData(text.GlowSansFontFile, control.Md5File, true)
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return
		}
		img, err = rendercard.DrawRankingCard(fontbyte, "猫猫排行榜", rankinfo)
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return
		}
		sendimg, err := imgfactory.ToBytes(img)
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return
		}
		if id := ctx.SendChain(message.ImageBytes(sendimg)); id.ID() == 0 {
			ctx.SendChain(message.Text("ERROR: 可能被风控了"))
		}
	})
}

func pkweight(player1, player2 *catInfo) (winer, loser *catInfo) {
	weightOfplayer1 := float64(100*player1.Breed+player1.Experience/10) + (player1.Weight-50)*0.05 +
		float64((player1.Mood-player2.Mood)*(player1.Mood-50))*0.4 +
		(player1.Satiety-player2.Satiety)*(player1.Satiety-50)*0.4
	weightOfplayer2 := float64(100*player2.Breed+player2.Experience/10) + (player2.Weight-50)*0.05 +
		float64((player2.Mood-player1.Mood)*(player2.Mood-50))*0.4 +
		(player2.Satiety-player1.Satiety)*(player2.Satiety-50)*0.4
	if weightOfplayer1 > weightOfplayer2 {
		return player1, player2
	}
	return player2, player1
}
