// Package ygo 一些关于ygo的插件
package ygo

import (
	"bytes"
	"image"
	"image/color"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"archive/zip"
	"encoding/json"

	"github.com/PuerkitoBio/goquery"

	"github.com/FloatTech/floatbox/binary"
	"github.com/FloatTech/floatbox/file"
	zbmath "github.com/FloatTech/floatbox/math"
	"github.com/FloatTech/floatbox/web"
	"github.com/FloatTech/imgfactory"
	ctrl "github.com/FloatTech/zbpctrl"
	control "github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension/single"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/FloatTech/gg"
	"github.com/FloatTech/zbputils/img/text"
)

const (
	serviceErr = "[ygocdb]error:"
	api        = "https://ygocdb.com/api/v0/?search="
	picherf    = "https://cdn.233.momobako.com/ygopro/pics/"
)

type searchResult struct {
	Result []cardInfo `json:"result"`
}

type cardInfo struct {
	Cid    int    `json:"cid"`
	ID     int    `json:"id"`
	CnName string `json:"cn_name"`
	ScName string `json:"sc_name"`
	MdName string `json:"md_name"`
	NwbbsN string `json:"nwbbs_n"`
	CnocgN string `json:"cnocg_n"`
	JpRuby string `json:"jp_ruby"`
	JpName string `json:"jp_name"`
	EnName string `json:"en_name"`
	Text   struct {
		Types string `json:"types"`
		Pdesc string `json:"pdesc"`
		Desc  string `json:"desc"`
	} `json:"text"`
	Data struct {
		Ot        int `json:"ot"`
		Setcode   int `json:"setcode"`
		Type      int `json:"type"`
		Atk       int `json:"atk"`
		Def       int `json:"def"`
		Level     int `json:"level"`
		Race      int `json:"race"`
		Attribute int `json:"attribute"`
	} `json:"data"`
}

type boxInfo struct {
	Time   string
	Number string
	Name   string
	Trade  string
}

// GameInfo 游戏信息
type GameInfo struct {
	UID         int64
	CID         int
	Name        string
	Pic         []byte
	Info        []string
	LastTime    time.Time // 距离上次回答时间
	Worry       int       // 错误次数
	TickCount   int       // 提示次数
	AnswerCount int       // 问答次数
}

var (
	en = control.Register("ygocdb", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "游戏王百鸽API", // 本插件基于游戏王百鸽API"https://www.ygo-sem.cn/"
		Help: "- /ydp [xxx]\n" +
			"- /yds [xxx]\n" +
			"- /ydb [xxx]\n" +
			"[xxx]为搜索内容\np:返回一张图片\ns:返回一张效果描述\nb:全显示" +
			"- /ys\n 随机分享一张卡" +
			"- 查[日版|英版|简中] [xxx]\n返回对应卡片的所有卡盒信息",
		PrivateDataFolder: "ygocdb",
	})
	zipfile       = en.DataFolder() + "ygocdb.com.cards.zip"
	verFile       = en.DataFolder() + "version.txt"
	cachePath     = en.DataFolder() + "pics/"
	lastVersion   = "123"
	lastTime      = 0
	lock          = sync.Mutex{}
	localJSONData = make(map[string]cardInfo)
	cradList      []string

	engine = control.Register("ygoguess", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "游戏王猜卡游戏", // 本插件基于游戏王百鸽API"https://www.ygo-sem.cn/"
		Help:             "- 猜卡游戏\n- 我猜xxx\n",
	}).ApplySingle(single.New(
		single.WithKeyFn(func(ctx *zero.Ctx) int64 { return ctx.Event.GroupID }),
		single.WithPostFn[int64](func(ctx *zero.Ctx) {
			ctx.Break()
			ctx.Send(
				message.ReplyWithMessage(ctx.Event.MessageID,
					message.Text("已经有正在进行的游戏..."),
				),
			)
		}),
	))
	gameRoom = make(map[int64]GameInfo, 100)
)

func init() {
	go func() {
		if file.IsNotExist(zipfile) {
			err := file.DownloadTo("https://ygocdb.com/api/v0/cards.zip", zipfile)
			if err != nil {
				panic(err)
			}
		}
		err := parsezip(zipfile)
		if err != nil {
			panic(err)
		}
		if file.IsNotExist(verFile) {
			data, err := web.GetData("https://ygocdb.com/api/v0/cards.zip.md5?callback=gu")
			if err != nil {
				panic(err)
			}
			lastTime = time.Now().Day()
			lastVersion = binary.BytesToString(data)
			fileData := binary.StringToBytes(strconv.Itoa(lastTime) + "\n" + lastVersion)
			err = os.WriteFile(verFile, fileData, 0644)
			if err != nil {
				panic(err)
			}
		} else {
			data, err := os.ReadFile(verFile)
			if err != nil {
				panic(err)
			}
			info := strings.Split(binary.BytesToString(data), "\n")
			time, err := strconv.Atoi(info[0])
			if err != nil {
				panic(err)
			}
			lastTime = time
			lastVersion = info[1]
		}
		err = os.MkdirAll(cachePath, 0755)
		if err != nil {
			panic(err)
		}
	}()
	en.OnRegex(`^/yd(p|s|b)\s?(.*)`).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		function := ctx.State["regex_matched"].([]string)[1]
		ctxtext := ctx.State["regex_matched"].([]string)[2]
		if ctxtext == "" {
			ctx.SendChain(message.Text("你是想查询「空手假象」吗？"))
			return
		}
		data, err := web.GetData(api + url.QueryEscape(ctxtext))
		if err != nil {
			ctx.SendChain(message.Text(serviceErr, err))
			return
		}
		var result searchResult
		err = json.Unmarshal(data, &result)
		if err != nil {
			ctx.SendChain(message.Text(serviceErr, err))
			return
		}
		maxpage := len(result.Result)
		switch {
		case maxpage == 0:
			ctx.SendChain(message.Text("没有找到相关的卡片额"))
			return
		case function == "p":
			ctx.SendChain(message.Image(picherf + strconv.Itoa(result.Result[0].ID) + ".jpg"))
			return
		case function == "s":
			cardtextout := cardtext(result, 0)
			ctx.SendChain(message.Text(cardtextout))
			return
		case function == "d" && maxpage == 1:
			cardtextout := cardtext(result, 0)
			ctx.SendChain(message.Image(picherf+strconv.Itoa(result.Result[0].ID)+".jpg"), message.Text(cardtextout))
			return
		}
		var listName []string
		var listid []int
		for _, v := range result.Result {
			listName = append(listName, strconv.Itoa(len(listName))+"."+v.CnName)
			listid = append(listid, v.ID)
		}
		var (
			currentPage = 10
			nextpage    = 0
		)
		if maxpage < 10 {
			currentPage = maxpage
		}
		ctx.SendChain(message.Text("找到", strconv.Itoa(maxpage), "张相关卡片,当前显示以下卡名：\n",
			strings.Join(listName[:currentPage], "\n"),
			"\n————————————\n输入对应数字获取卡片信息,",
			"\n或回复“取消”、“下一页”指令"))
		recv, cancel := zero.NewFutureEvent("message", 999, false, zero.RegexRule(`(取消)|(下一页)|\d+`), zero.OnlyGroup, zero.CheckUser(ctx.Event.UserID)).Repeat()
		after := time.NewTimer(20 * time.Second)
		for {
			select {
			case <-after.C:
				cancel()
				ctx.Send(
					message.ReplyWithMessage(ctx.Event.MessageID,
						message.Text("等待超时,搜索结束"),
					),
				)
				return
			case e := <-recv:
				nextcmd := e.Event.Message.String()
				switch nextcmd {
				case "取消":
					cancel()
					after.Stop()
					ctx.Send(
						message.ReplyWithMessage(ctx.Event.MessageID,
							message.Text("用户取消,搜索结束"),
						),
					)
					return
				case "下一页":
					after.Reset(20 * time.Second)
					if maxpage < 11 {
						continue
					}
					nextpage++
					if nextpage*10 >= maxpage {
						nextpage = 0
						currentPage = 10
						ctx.SendChain(message.Text("已是最后一页，返回到第一页"))
					} else if nextpage == maxpage/10 {
						currentPage = maxpage % 10
					}
					ctx.SendChain(message.Text("找到", strconv.Itoa(maxpage), "张相关卡片,当前显示以下卡名：\n",
						strings.Join(listName[nextpage*10:nextpage*10+currentPage], "\n"),
						"\n————————————————\n输入对应数字获取卡片信息,",
						"\n或回复“取消”、“下一页”指令"))
				default:
					cardint, err := strconv.Atoi(nextcmd)
					switch {
					case err != nil:
						after.Reset(20 * time.Second)
						ctx.SendChain(message.At(ctx.Event.UserID), message.Text("请输入正确的序号"))
					default:
						if cardint < nextpage*10+currentPage {
							cancel()
							after.Stop()
							cardtextout := cardtext(result, cardint)
							ctx.SendChain(message.Image(picherf+strconv.Itoa(listid[cardint])+".jpg"), message.Text(cardtextout))
							return
						}
						after.Reset(20 * time.Second)
						ctx.SendChain(message.At(ctx.Event.UserID), message.Text("请输入正确的序号"))
					}
				}
			}
		}
	})
	en.OnFullMatch("ycb更新").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		data, err := web.GetData("https://ygocdb.com/api/v0/cards.zip.md5?callback=gu")
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
		}
		version := binary.BytesToString(data)
		if version != lastVersion {
			err := file.DownloadTo("https://ygocdb.com/api/v0/cards.zip", zipfile)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			err = parsezip(zipfile)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			lastTime = time.Now().Day()
			lastVersion = version
			fileData := binary.StringToBytes(strconv.Itoa(lastTime) + "\n" + lastVersion)
			err = os.WriteFile(verFile, fileData, 0644)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			ctx.SendChain(message.At(ctx.Event.UserID), message.Text("更新成功"))
			return
		}
		ctx.SendChain(message.At(ctx.Event.UserID), message.Text("没发现更新内容"))
	})
	en.OnFullMatchGroup([]string{"分享卡片", "/ys"}, func(ctx *zero.Ctx) bool {
		lock.Lock()
		defer lock.Unlock()
		if time.Now().Day() == lastTime {
			return true
		}
		data, err := web.GetData("https://ygocdb.com/api/v0/cards.zip.md5?callback=gu")
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
		}
		version := binary.BytesToString(data)
		if version != lastVersion {
			err := file.DownloadTo("https://ygocdb.com/api/v0/cards.zip", zipfile)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return false
			}
			err = parsezip(zipfile)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return false
			}
			lastTime = time.Now().Day()
			lastVersion = version
			fileData := binary.StringToBytes(strconv.Itoa(lastTime) + "\n" + lastVersion)
			err = os.WriteFile(verFile, fileData, 0644)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return false
			}
			ctx.SendChain(message.Text("数据库已更新至最新"))
		}
		return true
	}).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		// 分享卡片
		data := drawCard()
		pic, err := drawimage(data)
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return
		}
		if ctx.ExtractPlainText() == "分享卡片" {
			ctx.SendChain(message.Text("今日分享卡片:"))
		}
		ctx.SendChain(message.ImageBytes(pic))
	})
	en.OnRegex(`^查(日版|英版|简中)卡盒\s*(.+)`).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		boxType := ctx.State["regex_matched"].([]string)[1]
		cardName := ctx.State["regex_matched"].([]string)[2]

		data, err := web.GetData(api + url.QueryEscape(cardName))
		if err != nil {
			ctx.SendChain(message.Text(serviceErr, err))
			return
		}
		var result searchResult
		err = json.Unmarshal(data, &result)
		if err != nil {
			ctx.SendChain(message.Text(serviceErr, err))
			return
		}
		if len(result.Result) == 0 {
			ctx.SendChain(message.Text("没有找到相关的卡片额"))
			return
		}
		card := result.Result[0]
		cid := strconv.Itoa(card.Cid)
		boxList := make([]boxInfo, 0, 256)
		switch boxType {
		case "日版": // 请求html页面
			res, err := http.Get("https://www.db.yugioh-card.com/yugiohdb/card_search.action?ope=2&request_locale=ja&cid=" + url.QueryEscape(cid))
			if err != nil {
				ctx.SendChain(message.Text(serviceErr, err))
				return
			}
			defer res.Body.Close()
			if res.StatusCode != 200 {
				ctx.SendChain(message.Text("status code error: [", res.StatusCode, "]", res.Status))
			}
			doc, err := goquery.NewDocumentFromReader(res.Body)
			if err != nil {
				ctx.SendChain(message.Text(serviceErr, err))
				return
			}
			doc.Find(".t_body").First().Find(".t_row").Each(func(_ int, contentSelection *goquery.Selection) {
				info := boxInfo{
					Time:   strings.TrimSpace(contentSelection.Find(".time").Text()),
					Number: strings.TrimSpace(contentSelection.Find(".card_number").Text()),
					Name:   strings.TrimSpace(contentSelection.Find(".pack_name.flex_1").Text()),
					Trade:  strings.TrimSpace(contentSelection.Find("p").Text()),
				}
				boxList = append(boxList, info)
			})
		case "英版":
			res, err := http.Get("https://www.db.yugioh-card.com/yugiohdb/card_search.action?ope=2&request_locale=en&cid=" + url.QueryEscape(cid))
			if err != nil {
				ctx.SendChain(message.Text(serviceErr, err))
				return
			}
			defer res.Body.Close()
			if res.StatusCode != 200 {
				ctx.SendChain(message.Text("status code error: [", res.StatusCode, "]", res.Status))
			}
			doc, err := goquery.NewDocumentFromReader(res.Body)
			if err != nil {
				ctx.SendChain(message.Text(serviceErr, err))
				return
			}
			doc.Find(".t_body").First().Find(".t_row").Each(func(_ int, contentSelection *goquery.Selection) {
				info := boxInfo{
					Time:   strings.TrimSpace(contentSelection.Find(".time").Text()),
					Number: strings.TrimSpace(contentSelection.Find(".card_number").Text()),
					Name:   strings.TrimSpace(contentSelection.Find(".pack_name.flex_1").Text()),
					Trade:  strings.TrimSpace(contentSelection.Find("p").Text()),
				}
				boxList = append(boxList, info)
			})
		case "简中":
			// res, err := http.Get("https://db.yugioh-card-cn.com/card_detail.html?lang=cn&id=" + url.QueryEscape(cid))
			// if err != nil {
			// 	ctx.SendChain(message.Text(serviceErr, err))
			// 	return
			// }
			// defer res.Body.Close()
			// if res.StatusCode != 200 {
			// 	ctx.SendChain(message.Text("status code error: [", res.StatusCode, "]", res.Status))
			// }
			// doc, err := goquery.NewDocumentFromReader(res.Body)
			// if err != nil {
			// 	ctx.SendChain(message.Text(serviceErr, err))
			// 	return
			// }
			// doc.Find(".t_row").Each(func(_ int, contentSelection *goquery.Selection) {
			// 	info := boxInfo{
			// 		Time:   strings.TrimSpace(contentSelection.Find(".time").First().Text()),
			// 		Number: strings.TrimSpace(contentSelection.Find(".card_number").First().Text()),
			// 		Name:   strings.TrimSpace(contentSelection.Find("class='pack_name flex_1'").First().Text()),
			// 		Trade:  strings.TrimSpace(contentSelection.Find("p").First().Text()),
			// 	}
			// 	boxList = append(boxList, info)
			// })
			ctx.SendChain(message.Text("简中卡盒为动态网页,本人能力有限,暂不支持"))
			return
		}
		number := len(boxList)
		if number <= 0 {
			ctx.SendChain(message.Text("未找到相关卡盒,请确认卡名后重试"))
			return
		}
		/***********设置图片的大小和底色***********/
		fontSize := 50.0
		if number < 10 {
			number = 10
		}

		canvas := gg.NewContext(1, 1)
		data, err = file.GetLazyData(text.BoldFontFile, control.Md5File, true)
		if err != nil {
			ctx.SendChain(message.Text("[qqwife]ERROR: ", err))
		}
		canvas.SetRGB(0, 0, 0)
		if err = canvas.ParseFontFace(data, fontSize*2); err != nil {
			ctx.SendChain(message.Text("[qqwife]ERROR: ", err))
			return
		}
		_, cnh := canvas.MeasureString(card.CnName)
		if err = canvas.ParseFontFace(data, fontSize); err != nil {
			ctx.SendChain(message.Text("[qqwife]ERROR: ", err))
			return
		}
		_, wh := canvas.MeasureString("卡盒")
		if err = canvas.ParseFontFace(data, fontSize*3); err != nil {
			ctx.SendChain(message.Text("[qqwife]ERROR: ", err))
			return
		}
		_, bh := canvas.MeasureString("稀有度")
		ggH := 10 + wh + 10 + cnh + 10 + wh + 20 + float64(number)*(bh+30) + 10

		canvas = gg.NewContext(2000, int(ggH))
		canvas.SetRGB(1, 1, 1)
		canvas.Clear()

		canvas.SetRGB(0, 0, 0)
		if err = canvas.ParseFontFace(data, fontSize*2); err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return
		}
		canvas.DrawString(card.CnName, 10, 10+wh+10+cnh+10)

		if err = canvas.ParseFontFace(data, fontSize); err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return
		}
		canvas.DrawString(card.JpName, 10, 10+wh)
		canvas.DrawString(card.EnName, 10, 10+wh+10+cnh+10+wh+10)

		canvas.DrawLine(10, 10+wh+10+cnh+10+wh+10, 1990, 10+wh+10+cnh+10+wh+30)
		canvas.SetLineWidth(2.5)
		canvas.Stroke()

		th := 10 + wh + 10 + cnh + 10 + wh + 10
		for i, info := range boxList {
			if err = canvas.ParseFontFace(data, fontSize*3); err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			canvas.SetRGB(0, 0, 0)
			canvas.DrawStringAnchored(info.Trade, 10+500/2, th+float64(i+1)*(bh+30)+10, 0.5, 0)

			if err = canvas.ParseFontFace(data, fontSize); err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			canvas.SetRGB(0, 0, 0)
			canvas.DrawString(info.Name, 10+500+30, th+float64(i)*(bh+30)+40+wh)
			canvas.DrawString("发售日期: "+info.Time+"    卡盒号:"+info.Number, 10+500+30, th+float64(i)*(bh+30)+40+wh+20+wh)

			canvas.DrawLine(10, th+float64(i+1)*(bh+30)+30, 1990, th+float64(i+1)*(bh+30)+30)
			canvas.SetLineWidth(2.5)
			canvas.Stroke()
		}
		data, err = imgfactory.ToBytes(canvas.Image())
		if err != nil {
			ctx.SendChain(message.Text("[qqwife]ERROR: ", err))
			return
		}
		ctx.SendChain(message.ImageBytes(data))
	})

	engine.OnRegex("^(黑边|反色|马赛克|旋转|切图)?猜卡游戏$", zero.OnlyGroup).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		lock.Lock()
		gid := ctx.Event.GroupID
		ctx.SendChain(message.Text("正在准备题目,请稍等"))
		gameInfo, ok := gameRoom[gid]
		if !ok || time.Since(gameInfo.LastTime).Seconds() > 105 {
			data := drawCard()
			picFile := cachePath + strconv.Itoa(data.ID) + ".jpg"
			if file.IsNotExist(picFile) {
				url := picherf + strconv.Itoa(data.ID) + ".jpg"
				err := file.DownloadTo(url, picFile)
				if err != nil {
					ctx.SendChain(message.Text("图片下载失败,可能被风控", err))
					return
				}
			}
			// 对卡图做处理
			pictrue, err := randPicture(picFile, data.Text.Types)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]", err))
				return
			}
			gameInfo = GameInfo{
				UID:      ctx.Event.UserID,
				CID:      data.ID,
				Name:     data.CnName,
				LastTime: time.Now(),
				Pic:      pictrue,
				Info:     []string{getTips(data, 0), getTips(data, 1), getTips(data, 2)},
			}
			gameRoom[gid] = gameInfo
		}
		lock.Unlock()
		picPath := cachePath + strconv.Itoa(gameInfo.CID) + ".jpg"
		pic, err := os.ReadFile(picPath)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]", err))
			return
		}
		length := zbmath.Ceil(len([]rune(gameInfo.Name)), 4)
		mid := ctx.SendChain(message.ImageBytes(gameInfo.Pic))
		if mid.ID() != 0 {
			ctx.SendChain(message.Text("请回答该图的卡名\n以“我猜xxx”格式回答\n(xxx需包含卡名1/4以上)\n或发“提示”得提示;“取消”结束游戏"))
		}
		// 进行猜卡环节
		recv, cancel := zero.NewFutureEvent("message", 1, true, zero.RegexRule("^((我猜.+)|提示|取消)$"), zero.OnlyGroup, zero.CheckGroup(ctx.Event.GroupID)).Repeat()
		defer cancel()
		subtime := time.Since(gameInfo.LastTime)
		if subtime.Seconds() > 105 {
			subtime = 0
		}
		tick := time.NewTimer(105*time.Second - subtime)
		over := time.NewTimer(120*time.Second - subtime)
		for {
			select {
			case <-tick.C:
				tick.Stop()
				ctx.SendChain(message.Text("还有15s作答时间"))
			case <-over.C:
				tick.Stop()
				over.Stop()
				msgID := ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID,
					message.Text("时间超时,游戏结束\n卡名是:\n", gameInfo.Name, "\n"),
					message.ImageBytes(pic)))
				if msgID.ID() == 0 {
					ctx.SendChain(message.Text("时间超时,游戏结束\n图片发送失败,可能被风控\n答案是:", gameInfo.Name))
				}
				delete(gameRoom, gid)
				return
			case c := <-recv:
				time.Sleep(time.Millisecond * time.Duration(10+rand.Intn(50)))
				msgID := c.Event.MessageID
				answer := c.Event.Message.String()
				_, after, ok := strings.Cut(answer, "我猜")
				if ok {
					if len([]rune(after)) < length {
						ctx.Send(message.ReplyWithMessage(msgID, message.Text("请输入", length, "字以上")))
						continue
					}
					answer = after
				}
				switch {
				case answer == "取消":
					if c.Event.UserID != ctx.Event.UserID {
						ctx.Send(message.ReplyWithMessage(msgID, message.Text("你无权限取消")))
						continue
					}
					tick.Stop()
					over.Stop()
					gameInfo.Worry = zbmath.Max(gameInfo.Worry, 6)
					msgID := ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID,
						message.Text("游戏已取消\n卡名是:\n", gameInfo.Name, "\n"),
						message.ImageBytes(pic)))
					if msgID.ID() == 0 {
						ctx.SendChain(message.Text("游戏已取消\n图片发送失败,可能被风控\n答案是:", gameInfo.Name))
					}
					delete(gameRoom, gid)
					return
				case answer == "提示" && gameInfo.TickCount > 2:
					tick.Reset(105 * time.Second)
					over.Reset(120 * time.Second)
					ctx.Send(message.ReplyWithMessage(msgID, message.Text("已经没有提示了哦,加油啊")))
					continue
				case answer == "提示":
					// gameInfo.Worry++
					tick.Reset(105 * time.Second)
					over.Reset(120 * time.Second)
					ctx.Send(message.ReplyWithMessage(msgID, message.Text(gameInfo.Info[gameInfo.TickCount])))
					gameInfo.TickCount++
					continue
				case strings.Contains(gameInfo.Name, answer):
					tick.Stop()
					over.Stop()
					msgID := ctx.Send(message.ReplyWithMessage(msgID,
						message.Text("太棒了,你猜对了!\n卡名是:\n", gameInfo.Name, "\n"),
						message.ImageBytes(pic)))
					if msgID.ID() == 0 {
						ctx.SendChain(message.Text("太棒了,你猜对了!\n图片发送失败,可能被风控\n答案是:", gameInfo.Name))
					}
					delete(gameRoom, gid)
					return
				case gameInfo.AnswerCount >= 5:
					tick.Stop()
					over.Stop()
					msgID := ctx.Send(message.ReplyWithMessage(msgID,
						message.Text("次数到了,很遗憾没能猜出来\n卡名是:\n", gameInfo.Name, "\n"),
						message.ImageBytes(pic)))
					if msgID.ID() == 0 {
						ctx.SendChain(message.Text("次数到了,很遗憾没能猜出来\n图片发送失败,可能被风控\n答案是:", gameInfo.Name))
					}
					delete(gameRoom, gid)
					return
				default:
					gameInfo.Worry++
					gameInfo.AnswerCount++
					tick.Reset(105 * time.Second)
					over.Reset(120 * time.Second)
					ctx.Send(message.ReplyWithMessage(msgID, message.Text("答案不对哦,还有"+strconv.Itoa(6-gameInfo.AnswerCount)+"次回答机会,加油啊~")))
				}
			}
		}
	})
}

func cardtext(list searchResult, cardid int) string {
	var cardtext []string
	cardtext = append(cardtext, "中文卡名：\n    "+list.Result[cardid].CnName)
	if list.Result[cardid].JpName == "" {
		cardtext = append(cardtext, "英文卡名：\n    "+list.Result[cardid].EnName)
	} else {
		cardtext = append(cardtext, "日文卡名：\n    "+list.Result[cardid].JpName)
	}
	cardtext = append(cardtext, "卡片密码："+strconv.Itoa(list.Result[cardid].ID))
	cardtext = append(cardtext, list.Result[cardid].Text.Types)
	if list.Result[cardid].Text.Pdesc != "" {
		cardtext = append(cardtext, "[灵摆效果]\n"+list.Result[cardid].Text.Pdesc)
		if strings.Contains(list.Result[cardid].Text.Types, "效果") {
			cardtext = append(cardtext, "[怪兽效果]")
		} else {
			cardtext = append(cardtext, "[怪兽描述]")
		}
	}
	cardtext = append(cardtext, list.Result[cardid].Text.Desc)
	return strings.Join(cardtext, "\n")
}

func parsezip(zipFile string) error {
	zipReader, err := zip.OpenReader(zipFile) // will not close
	if err != nil {
		return err
	}
	defer zipReader.Close()
	file, err := zipReader.File[0].Open()
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &localJSONData)
	if err != nil {
		return err
	}
	cradList = []string{}
	for key := range localJSONData {
		cradList = append(cradList, key)
	}
	return nil
}

func drawCard() cardInfo {
	data := cardInfo{}
	max := len(cradList)
	if max > 0 {
		data = localJSONData[cradList[rand.Intn(max)]]
	}
	return data
}

// 绘制图片
func drawimage(info cardInfo) (data []byte, err error) {
	byteData, err := web.GetData(picherf + strconv.Itoa(info.ID) + ".jpg")
	if err != nil {
		return
	}
	// 卡图大小
	cardPic, _, err := image.Decode(bytes.NewReader(byteData))
	if err != nil {
		return
	}
	cardPic = imgfactory.Size(cardPic, 400, 580).Image()
	picx := cardPic.Bounds().Dx()
	picy := cardPic.Bounds().Dy()

	canvas := gg.NewContext(1, 1)
	data, err = file.GetLazyData(text.BoldFontFile, control.Md5File, true)
	if err != nil {
		return
	}
	if err = canvas.ParseFontFace(data, 50); err != nil {
		return
	}
	var cardBaseInfo []string
	name := "C N卡名: " + info.CnName
	baseW, _ := canvas.MeasureString(name)
	cardBaseInfo = append(cardBaseInfo, name)
	if info.NwbbsN != "" {
		name = "N W卡名: " + info.NwbbsN
		nW, _ := canvas.MeasureString(name)
		if nW > baseW {
			baseW = nW
		}
		cardBaseInfo = append(cardBaseInfo, name)
	}
	if info.CnocgN != "" {
		name = "简中卡名: " + info.CnocgN
		nW, _ := canvas.MeasureString(name)
		if nW > baseW {
			baseW = nW
		}
		cardBaseInfo = append(cardBaseInfo, name)
	}
	if info.NwbbsN != "" {
		name = "M D卡名: " + info.MdName
		nW, _ := canvas.MeasureString(name)
		if nW > baseW {
			baseW = nW
		}
		cardBaseInfo = append(cardBaseInfo, name)
	}
	if info.JpName != "" {
		name = "日本卡名:"
		if info.JpRuby != "" && info.JpName != info.JpRuby {
			name += "\n    " + info.JpRuby
			nW, _ := canvas.MeasureString("    " + info.JpRuby)
			if nW > baseW {
				baseW = nW
			}
		}
		name += "\n    " + info.JpName
		nW, _ := canvas.MeasureString("    " + info.JpName)
		if nW > baseW {
			baseW = nW
		}
		cardBaseInfo = append(cardBaseInfo, name)
	}
	if info.EnName != "" {
		name = "英文卡名:\n    " + info.EnName
		nW, _ := canvas.MeasureString("    " + info.EnName)
		if nW > baseW {
			baseW = nW
		}
		cardBaseInfo = append(cardBaseInfo, name)
	}
	if info.ScName != "" {
		name = "其他译名: " + info.ScName
		nW, _ := canvas.MeasureString(name)
		if nW > baseW {
			baseW = nW
		}
		cardBaseInfo = append(cardBaseInfo, name)
	}
	name = "卡片密码：" + strconv.Itoa(info.ID)
	nW, _ := canvas.MeasureString(name)
	if nW > baseW {
		baseW = nW
	}
	cardBaseInfo = append(cardBaseInfo, name)
	name = info.Text.Types
	nW, _ = canvas.MeasureString(name)
	if nW > baseW {
		baseW = nW
	}
	cardBaseInfo = append(cardBaseInfo, name)
	baseTextPic, err := text.Render(strings.Join(cardBaseInfo, "\n"), text.BoldFontFile, int(baseW), 50)
	if err != nil {
		return
	}
	basePicPx := baseTextPic.Bounds().Dx()
	basePicPy := baseTextPic.Bounds().Dy()

	textWidth := basePicPx + 10 + picx
	decsPic := []string{}
	if info.Text.Pdesc != "" {
		decsPic = append(decsPic, "[灵摆效果]\n"+info.Text.Pdesc)
		if strings.Contains(info.Text.Types, "效果") {
			decsPic = append(decsPic, "[怪兽效果]")
		} else {
			decsPic = append(decsPic, "[怪兽描述]")
		}
	}
	decsPic = append(decsPic, info.Text.Desc)
	textPic, err := text.Render(strings.Join(decsPic, "\n"), text.BoldFontFile, textWidth, 50)
	if err != nil {
		return
	}
	decsPicPx := textPic.Bounds().Dx()
	decsPicPy := textPic.Bounds().Dy()

	h := zbmath.Max(picy, basePicPy)
	canvas = gg.NewContext(10+decsPicPx+10, 10+h+10+decsPicPy)
	canvas.SetRGB(1, 1, 1)
	canvas.Clear()
	// 放置效果
	canvas.DrawImage(baseTextPic, 10, 10)
	// 放置效果
	canvas.DrawImage(cardPic, 10+basePicPx+10, 10)
	// 放置效果
	canvas.DrawImage(textPic, 10, h+10)
	// 生成图片
	data, err = imgfactory.ToBytes(canvas.Image())
	return
}

// 随机选择
func randPicture(picFile, cardType string) ([]byte, error) {
	types := []func(*imgfactory.Factory) ([]byte, error){
		backPic, mosaic, doublePicture, cutPic, randSet,
	}
	pic, err := gg.LoadImage(picFile)
	if err != nil {
		return nil, err
	}
	dst := imgfactory.Size(pic, pic.Bounds().Dx(), pic.Bounds().Dy())
	if strings.Contains(cardType, "灵摆") {
		dst = dst.Clip(370-29, 358-105, 29, 105)
	} else {
		dst = dst.Clip(351-51, 408-108, 51, 108)
	}
	dst = imgfactory.Size(dst.Image(), 256*5, 256*5)
	id := rand.Intn(len(types))
	println("\n*********猜卡ID:", id, " *********\n")
	dstfunc := types[id]
	picbytes, err := dstfunc(dst)
	return picbytes, err
}

// 获取黑边
func backPic(dst *imgfactory.Factory) ([]byte, error) {
	bounds := dst.Image().Bounds()
	returnpic := imgfactory.NewFactoryBG(dst.W(), dst.H(), color.NRGBA{255, 255, 255, 255}).Image()

	for y := bounds.Min.Y; y <= bounds.Max.Y; y++ {
		for x := bounds.Min.X; x <= bounds.Max.X; x++ {
			a := dst.Image().At(x, y)
			colorA := color.NRGBAModel.Convert(a).(color.NRGBA)
			b := dst.Image().At(x+1, y)
			colorB := color.NRGBAModel.Convert(b).(color.NRGBA)
			c := dst.Image().At(x, y+1)
			colorC := color.NRGBAModel.Convert(c).(color.NRGBA)
			if math.Sqrt(float64((colorA.R-colorB.R)*(colorA.R-colorB.R)+(colorA.G-colorB.G)*(colorA.G-colorB.G)+(colorA.B-colorB.B)*(colorA.B-colorB.B))) > 15 {
				returnpic.Set(x, y, color.NRGBA{0, 0, 0, 0})
			} else if math.Sqrt(float64((colorA.R-colorC.R)*(colorA.R-colorC.R)+(colorA.G-colorC.G)*(colorA.G-colorC.G)+(colorA.B-colorC.B)*(colorA.B-colorC.B))) > 15 {
				returnpic.Set(x, y, color.NRGBA{0, 0, 0, 0})
			}
		}
	}
	return imgfactory.ToBytes(returnpic)
}

// 旋转
func doublePicture(dst *imgfactory.Factory) ([]byte, error) {
	b := dst.Image().Bounds()
	pic := dst.FlipH().FlipV()
	for y := b.Min.Y; y <= b.Max.Y; y++ {
		for x := b.Min.X; x <= b.Max.X; x++ {
			a := pic.Image().At(x, y)
			c := color.NRGBAModel.Convert(a).(color.NRGBA)
			a1 := dst.Image().At(x, y)
			c1 := color.NRGBAModel.Convert(a1).(color.NRGBA)
			switch {
			case y > x && x < b.Max.X/2 && y < b.Max.Y/2:
				dst.Image().Set(x, y, c)
			case y < x && x > b.Max.X/2 && y > b.Max.Y/2:
				dst.Image().Set(x, y, c)
			case y > b.Max.Y-x && x < b.Max.X/2 && y > b.Max.Y/2:
				dst.Image().Set(x, y, c)
			case y < b.Max.Y-x && x > b.Max.X/2 && y < b.Max.Y/2:
				dst.Image().Set(x, y, c)
			default:
				dice := rand.Intn(10)
				if dice < 3 {
					dst.Image().Set(x, y, color.NRGBA{
						R: c1.R,
						G: c1.G,
						B: c1.B,
						A: 255,
					})
				} else {
					dst.Image().Set(x, y, color.NRGBA{
						R: 255,
						G: 255,
						B: 255,
						A: 255,
					})
				}
			}
		}
	}
	return imgfactory.ToBytes(dst.Image())
}

// 马赛克
func mosaic(dst *imgfactory.Factory) ([]byte, error) {
	b := dst.Image().Bounds()
	markSize := (b.Max.X * (5 - rand.Intn(4))) / 100

	for yOfMarknum := 0; yOfMarknum <= zbmath.Ceil(b.Max.Y, markSize); yOfMarknum++ {
		for xOfMarknum := 0; xOfMarknum <= zbmath.Ceil(b.Max.X, markSize); xOfMarknum++ {
			a := dst.Image().At(xOfMarknum*markSize+markSize/2, yOfMarknum*markSize+markSize/2)
			cc := color.NRGBAModel.Convert(a).(color.NRGBA)
			for y := 0; y < markSize; y++ {
				for x := 0; x < markSize; x++ {
					xOfPic := xOfMarknum*markSize + x
					yOfPic := yOfMarknum*markSize + y
					dst.Image().Set(xOfPic, yOfPic, cc)
				}
			}
		}
	}
	return imgfactory.ToBytes(dst.Blur(3).Image())
}

// 随机切割
func cutPic(dst *imgfactory.Factory) ([]byte, error) {
	indexOfx := rand.Intn(3)
	indexOfy := rand.Intn(3)
	indexOfx2 := rand.Intn(3)
	indexOfy2 := rand.Intn(3)
	b := dst.Image()
	bx := b.Bounds().Max.X / 3
	by := b.Bounds().Max.Y / 3
	returnpic := imgfactory.NewFactoryBG(dst.W(), dst.H(), color.NRGBA{255, 255, 255, 255})

	for yOfMarknum := b.Bounds().Min.Y; yOfMarknum <= b.Bounds().Max.Y; yOfMarknum++ {
		for xOfMarknum := b.Bounds().Min.X; xOfMarknum <= b.Bounds().Max.X; xOfMarknum++ {
			if xOfMarknum == bx || yOfMarknum == by || xOfMarknum == bx*2 || yOfMarknum == by*2 {
				// 黑框
				returnpic.Image().Set(xOfMarknum, yOfMarknum, color.NRGBA{0, 0, 0, 0})
			}
			if xOfMarknum >= bx*indexOfx && xOfMarknum < bx*(indexOfx+1) {
				if yOfMarknum >= by*indexOfy && yOfMarknum < by*(indexOfy+1) {
					a := dst.Image().At(xOfMarknum, yOfMarknum)
					cc := color.NRGBAModel.Convert(a).(color.NRGBA)
					returnpic.Image().Set(xOfMarknum, yOfMarknum, cc)
				}
			}
			if xOfMarknum >= bx*indexOfx2 && xOfMarknum < bx*(indexOfx2+1) {
				if yOfMarknum >= by*indexOfy2 && yOfMarknum < by*(indexOfy2+1) {
					a := dst.Image().At(xOfMarknum, yOfMarknum)
					cc := color.NRGBAModel.Convert(a).(color.NRGBA)
					returnpic.Image().Set(xOfMarknum, yOfMarknum, cc)
				}
			}
		}
	}
	return imgfactory.ToBytes(returnpic.Image())
}

// 乱序
func randSet(dst *imgfactory.Factory) ([]byte, error) {
	b := dst.Image().Bounds()
	w, h := b.Max.X/3, b.Max.Y/3
	returnpic := imgfactory.NewFactoryBG(dst.W(), dst.H(), color.NRGBA{255, 255, 255, 255})

	mapPicOfX := []int{0, 0, 0, 1, 1, 1, 2, 2, 2}
	mapPicOfY := []int{0, 1, 2, 0, 1, 2, 0, 1, 2}

	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			index := 0
			mapfaceX := mapPicOfX[index]
			mapfaceY := mapPicOfY[index]
			if len(mapPicOfX) > 1 {
				index = rand.Intn(len(mapPicOfX))
				mapfaceX = mapPicOfX[index]
				mapfaceY = mapPicOfY[index]
				mapPicOfX = append(mapPicOfX[:index], mapPicOfX[index+1:]...)
				mapPicOfY = append(mapPicOfY[:index], mapPicOfY[index+1:]...)
			}
			for x := 0; x < w; x++ {
				for y := 0; y < h; y++ {
					a := dst.Image().At(mapfaceX*w+x, mapfaceY*h+y)
					cc := color.NRGBAModel.Convert(a).(color.NRGBA)
					returnpic.Image().Set(i*w+x, j*h+y, cc)
				}
			}
		}
	}
	return imgfactory.ToBytes(returnpic.Image())
}

// 拼接提示词
func getTips(cardData cardInfo, quitCount int) string {
	name := []rune(cardData.CnName)
	switch quitCount {
	case 0:
		typeInfo, _, _ := strings.Cut(cardData.Text.Types, "]")
		return "这是一张" + typeInfo + "],卡名是" + strconv.Itoa(len(name)) + "字的"
	case 2:
		if len(name) <= 1 {
			return "这是一张" + cardData.Text.Types
		}
		return "卡名含有: " + string(name[rand.Intn(len(name))])
	default:
		text := cardData.Text.Desc + cardData.Text.Pdesc
		textrand := []string{cardData.Text.Types}
		listmax := regexp.MustCompile(`(「.+」)`).FindAllStringSubmatch(text, -1)
		for _, value := range listmax {
			text = strings.ReplaceAll(text, value[0], "「xxx」")
		}
		depict := strings.Split(text, "。")
		for _, value := range depict {
			value = strings.TrimSpace(value)
			// value = strings.Replace(value, "\n", "", -1)
			if value != "" {
				list := strings.Split(value, "，")
				for _, value2 := range list {
					if value2 != "" {
						textrand = append(textrand, value2)
					}
				}
			}
		}
		return textrand[rand.Intn(len(textrand))]
	}
}
