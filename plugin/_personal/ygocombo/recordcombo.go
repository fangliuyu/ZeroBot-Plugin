// Package sleepmanage 睡眠管理
package recordcombo

import (
	"fmt"
	"strconv"
	"time"

	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/FloatTech/floatbox/binary"
	ctrl "github.com/FloatTech/zbpctrl"
	control "github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/img/text"
	log "github.com/sirupsen/logrus"
)

func init() { // 插件主体
	engine := control.Register("recordcombo", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Help: "combo记录器\n" +
			"- combo列表\n" +
			"- 回复要记录的combo内容对话“记录combo combo名称”\n" +
			"- 查看combo [xxx]\n" +
			"- 随机combo [xxx]\n" +
			"- 删除combo [xxx]  (仅管理员可用)\n",
		PublicDataFolder: "YgoCombo",
	})
	sdb = initialize(engine.DataFolder() + "combo.db")
	engine.OnRegex(`^\[CQ:reply,id=.*](\s+)?记录combo(\s(.*))?`).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		Message := ctx.GetMessage(message.NewMessageIDFromString(ctx.Event.Message[0].Data["id"]))
		combocontent := Message.Elements.String() // combo内容
		if combocontent == "" {
			ctx.Send(
				message.ReplyWithMessage(ctx.Event.MessageID,
					message.Text("你是想记录「空手假象」combo吗？"),
				),
			)
			return
		}
		comboName := ctx.State["regex_matched"].([]string)[2]
		if comboName == "" {
			ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("请输入combo名称")))
			// 等待用户下一步选择
			recv, cancel := zero.NewFutureEvent("message", 999, false, ctx.CheckSession()).Repeat()
			for {
				select {
				case <-time.After(time.Second * 30): // 两分钟等待
					cancel()
					ctx.Send(
						message.ReplyWithMessage(ctx.Event.MessageID,
							message.Text("等待超时,记录失败"),
						),
					)
					return
				case e := <-recv:
					comboName = e.Event.Message.String()
					cancel()
					err := sdb.addmanage(comboName, ctx.Event.UserID, Message.Sender.ID, ctx.Event.GroupID, combocontent)
					if err != nil {
						ctx.SendChain(message.Text("ERROR:", err))
						return
					}
					msg := make(message.Message, 0, 3)
					msg = append(msg, message.Text("成功添加“", comboName, "”combo\n内容：\n"))
					msg = append(msg, message.ParseMessageFromString(combocontent)...)
					ctx.Send(msg)
					return
				}
			}
		}
		err := sdb.addmanage(comboName, ctx.Event.UserID, Message.Sender.ID, ctx.Event.GroupID, combocontent)
		if err != nil {
			ctx.SendChain(message.Text("ERROR:", err))
			return
		}
		msg := make(message.Message, 0, 3)
		msg = append(msg, message.Text("成功添加“", comboName, "”combo\n内容：\n"))
		msg = append(msg, message.ParseMessageFromString(combocontent)...)
		ctx.Send(msg)
	})

	engine.OnRegex(`^删除combo (.+)$`, zero.SuperUserPermission).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			comboName := ctx.State["regex_matched"].([]string)[1]
			err := sdb.removemanage(comboName)
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			ctx.SendChain(message.Text(comboName, "删除成功"))
		})
	//
	engine.OnFullMatchGroup([]string{"combo列表"}).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			rows, state, err := sdb.managelist()
			if err != nil {
				ctx.SendChain(message.Text(err))
				return
			}
			msg := make([]any, 0, rows)
			for i := 0; i < rows; i++ {
				msg = append(msg,
					strconv.Itoa(i), ".combo名称：", state[i].ComboName,
					"\n    创建人：", ctx.CardOrNickName(state[i].CreateID),
					"\n           (", state[i].CreateID, ")\n",
					"    记录时间：", state[i].CreateData, "\n\n",
				)
			}
			data, err := text.RenderToBase64(fmt.Sprint(msg...), text.FontFile, 1500, 50)
			if err != nil {
				log.Errorf("[control] %v", err)
			}
			if id := ctx.SendChain(message.Image("base64://" + binary.BytesToString(data))); id.ID() == 0 {
				ctx.SendChain(message.Text("ERROR: 可能被风控了"))
			}
		})

	engine.OnPrefix("查看combo").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		comboName := ctx.State["args"].(string)
		state, err := sdb.lookupmanage(comboName)
		if err != nil {
			ctx.SendChain(message.Text(err))
			return
		}
		msg := make([]any, 0, 4)
		msg = append(msg,
			"combo名称：", state.ComboName,
			"\n创建人：", ctx.CardOrNickName(state.CreateID),
			"\n      (", state.CreateID, ")\n",
			"记录人：", ctx.CardOrNickName(state.UserID),
			"\n      (", state.UserID, ")\n",
		)
		if state.GroupID != 0 {
			msg = append(msg,
				"所在群：", ctx.GetGroupInfo(state.GroupID, false).Name,
				"\n      (", state.GroupID, ")\n",
			)
		}
		msg = append(msg,
			"创建时间：", state.CreateData,
		)
		data, err := text.RenderToBase64(fmt.Sprint(msg...), text.FontFile, 1500, 50)
		if err != nil {
			log.Errorf("[control] %v", err)
		}
		if id := ctx.SendChain(message.Image("base64://" + binary.BytesToString(data))); id.ID() == 0 {
			ctx.SendChain(message.Text("ERROR: 可能被风控了"))
		}
		ctx.Send(message.ParseMessageFromString(state.ComboContent))
	})

	engine.OnFullMatch("随机combo").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		state, err := sdb.randinfo()
		if err != nil {
			ctx.SendChain(message.Text(err))
			return
		}
		msg := make([]any, 0, 4)
		msg = append(msg,
			"combo名称：", state.ComboName,
			"\n创建人：", ctx.CardOrNickName(state.CreateID),
			"\n      (", state.CreateID, ")\n",
			"记录人：", ctx.CardOrNickName(state.UserID),
			"\n      (", state.UserID, ")\n",
		)
		if state.GroupID != 0 {
			msg = append(msg,
				"所在群：", ctx.GetGroupInfo(state.GroupID, false).Name,
				"\n      (", state.GroupID, ")\n",
			)
		}
		msg = append(msg,
			"创建时间：", state.CreateData,
		)
		data, err := text.RenderToBase64(fmt.Sprint(msg...), text.FontFile, 1500, 50)
		if err != nil {
			log.Errorf("[control] %v", err)
		}
		if id := ctx.SendChain(message.Image("base64://" + binary.BytesToString(data))); id.ID() == 0 {
			ctx.SendChain(message.Text("ERROR: 可能被风控了"))
		}
		ctx.Send(message.ParseMessageFromString(state.ComboContent))
	})
}
