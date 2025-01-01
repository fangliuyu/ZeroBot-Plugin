// Package aireply AI 回复
package aireply

import (
	"time"

	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var replmd = replymode([]string{"婧枫", "沫沫", "青云客", "小爱", "ChatGPT"})

func init() { // 插件主体
	enr := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault:  false,
		Brief:             "人工智能回复",
		Help:              "- @Bot 任意文本(任意一句话回复)\n- 设置文字回复模式[婧枫|沫沫|青云客|小爱|ChatGPT]\n- 设置 ChatGPT api key xxx",
		PrivateDataFolder: "aireply",
	})

	enr.OnMessage(zero.OnlyToMe).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(func(ctx *zero.Ctx) {
			aireply := replmd.getReplyMode(ctx)
			reply := message.ParseMessageFromString(aireply.Talk(ctx.Event.UserID, ctx.ExtractPlainText(), zero.BotConfig.NickName[0]))
			// 回复
			time.Sleep(time.Second * 1)
			reply = append(reply, message.Reply(ctx.Event.MessageID))
			ctx.Send(reply)
		})
	setReplyMode := func(ctx *zero.Ctx) {
		param := ctx.State["args"].(string)
		err := replmd.setReplyMode(ctx, param)
		if err != nil {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(err))
			return
		}
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("成功"))
	}
	enr.OnPrefix("设置文字回复模式", zero.AdminPermission).SetBlock(true).Handle(setReplyMode)
	enr.OnRegex(`^设置\s*ChatGPT\s*api\s*key\s*(.*)$`, zero.OnlyPrivate, zero.SuperUserPermission).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		err := ཆཏ.set(ctx.State["regex_matched"].([]string)[1])
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return
		}
		ctx.SendChain(message.Text("设置成功"))
	})
}
