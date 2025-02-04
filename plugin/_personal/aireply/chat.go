// Package aireply AI 回复
package aireply

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/FloatTech/floatbox/web"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

const (
	URL     = "https://azapi.top"
	apiURL  = URL + "/api/gpt.php?msg=%v&system=%v"
	apiName = "娮儿"
	presume = "你全名叫柳如娮,平常大家叫你娮儿,设定上是卡牌游戏游戏王的第七部番剧《游戏王SEVENS》中哥哈市第七小学的5年1班学生,王道游我的同班同学,也是社团“轻音部”的成员。成绩优秀、体育万能,同时也是小学生乐队“露亚露明”的成员,是吉他手雾岛露明的闺蜜,你很希望能和露明成为百合。受露明影响你也很喜欢上了游戏王决斗。喜欢可爱的怪兽。使用和露明一样的念动力族怪兽的卡组,王牌怪兽也是“彩光之首席女吉他手”。"
	ua      = "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Mobile Safari/537.36"
)

type apiData struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string      `json:"role"`
			Content string      `json:"content"`
			Refusal interface{} `json:"refusal"`
		} `json:"message"`
		Logprobs     interface{} `json:"logprobs"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		TotalTokens         int `json:"total_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
	SystemFingerprint string `json:"system_fingerprint"`
}

func init() { // 插件主体
	enr := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "人工智能回复",
		Help:             "- @Bot 任意文本(任意一句话回复)",
	})

	enr.OnMessage(zero.OnlyToMe).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(func(ctx *zero.Ctx) {
			msg := ctx.ExtractPlainText()
			replyMssg := talkPlain(msg)
			if replyMssg == "" {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("不知道,要不问一下百度?"))
				return
			}
			reply := message.ParseMessageFromString(replyMssg)
			// 回复
			time.Sleep(time.Second * 1)
			reply = append(reply, message.Reply(ctx.Event.MessageID))
			ctx.Send(reply)
		})
}

func talkPlain(msg string) string {
	for _, name := range zero.BotConfig.NickName {
		msg = strings.ReplaceAll(msg, name, apiName)
	}
	u := fmt.Sprintf(apiURL, url.QueryEscape(msg), url.QueryEscape(presume))
	data, err := web.GetData(u)
	if err != nil {
		logrus.Errorln(err)
		return ""
	}
	var result apiData
	err = json.Unmarshal(data, &result)
	if err != nil {
		logrus.Errorln(err)
		return ""
	}
	replystr := result.Choices[0].Message.Content
	replystr = strings.ReplaceAll(replystr, "<img src=\"", "[CQ:image,file=")
	replystr = strings.ReplaceAll(replystr, "<br>", "\n")
	replystr = strings.ReplaceAll(replystr, "\" />", "]")
	return replystr
}
