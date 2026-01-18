// Package ygo 一些关于ygo的插件
package ygo

import (
	"bytes"
	"image"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"archive/zip"
	"encoding/json"

	"github.com/PuerkitoBio/goquery"

	"github.com/FloatTech/floatbox/binary"
	fcext "github.com/FloatTech/floatbox/ctxext"
	"github.com/FloatTech/floatbox/file"
	zbmath "github.com/FloatTech/floatbox/math"
	"github.com/FloatTech/floatbox/web"
	"github.com/FloatTech/imgfactory"
	sql "github.com/FloatTech/sqlite"
	ctrl "github.com/FloatTech/zbpctrl"
	control "github.com/FloatTech/zbputils/control"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/FloatTech/gg"
	"github.com/FloatTech/zbputils/img/text"
)

const (
	serviceErr = "[ygocdb]error:"
	api        = "https://ygocdb.com/api/v0/?search="
	picherf    = "https://cdn.233.momobako.com/ygopro/pics/"
)

// ygoDB 继承方法的存储结构
type ygoDB struct {
	sync.RWMutex
	db sql.Sqlite
}

type subscribe struct {
	GID int64
}

/*
	type groupInfo struct {
		UID int64
		Answers int64 // 答题数
		FULL int64 // 满分数
		Accuracy int64 // 答对题数
		Integrity int64 // 完整度
	}
*/
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

var (
	ygocdb = control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "游戏王百鸽API", // 本插件基于游戏王百鸽API"https://www.ygo-sem.cn/"
		Help: "- /ydp [xxx]\n" +
			"- /yds [xxx]\n" +
			"- /ydb [xxx]\n" +
			"[xxx]为搜索内容\np:返回一张图片\ns:返回一张效果描述\nb:全显示" +
			"- /ys\n 随机分享一张卡" +
			"- 查[日版|英版|简中] [xxx]\n返回对应卡片的所有卡盒信息",
		PrivateDataFolder: "ygo",
	})
	zipfile       = ygocdb.DataFolder() + "ygocdb.com.cards.zip"
	verFile       = ygocdb.DataFolder() + "version.txt"
	cachePath     = ygocdb.DataFolder() + "pics/"
	lastVersion   = "123"
	lastTime      = 0
	lock          = sync.Mutex{}
	localJSONData = make(map[string]cardInfo)
	cradList      []string

	database ygoDB
	// 开启并检查数据库链接
	getDB = fcext.DoOnceOnSuccess(func(ctx *zero.Ctx) bool {
		database.db = sql.New(ygocdb.DataFolder() + "userdata.db")
		err := database.db.Open(24 * time.Hour)
		if err != nil {
			ctx.SendChain(message.Text(serviceErr, err))
			return false
		}
		if err = database.db.Create("subscribe", &subscribe{}); err != nil {
			ctx.SendChain(message.Text(serviceErr, err))
			return false
		}
		return true
	})
	checkUpdate = func(ctx *zero.Ctx) bool {
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
	}
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
	ygocdb.OnRegex(`^/yd(p|s|b)\s?(.*)`).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		function := ctx.State["regex_matched"].([]string)[1]
		ctxtext := ctx.State["regex_matched"].([]string)[2]
		if ctxtext == "" {
			ctx.SendChain(message.Text("你是想查询「空手假象」吗？"))
			return
		}
		result, err := GetCardInfo(ctxtext)
		if err != nil {
			ctx.SendChain(message.Text(serviceErr, err))
			return
		}
		maxpage := len(result)
		switch {
		case maxpage == 0:
			ctx.SendChain(message.Text("没有找到相关的卡片额"))
			return
		case function == "p":
			ctx.SendChain(message.Image(picherf + strconv.Itoa(result[0].ID) + ".jpg"))
			return
		case function == "s":
			cardtextout := Cardtext(result[0])
			ctx.SendChain(message.Text(cardtextout))
			return
		case function == "b" && maxpage == 1:
			pic, err := drawimage(result[0])
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			ctx.SendChain(message.ImageBytes(pic))
			return
		}
		var listName []string
		for _, v := range result {
			listName = append(listName, strconv.Itoa(len(listName))+"."+v.CnName)
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
							pic, err := drawimage(result[cardint])
							if err != nil {
								ctx.SendChain(message.Text("ERROR: ", err))
								return
							}
							ctx.SendChain(message.ImageBytes(pic))
							return
						}
						after.Reset(20 * time.Second)
						ctx.SendChain(message.At(ctx.Event.UserID), message.Text("请输入正确的序号"))
					}
				}
			}
		}
	})
	ygocdb.OnFullMatch("ycb更新").SetBlock(true).Handle(func(ctx *zero.Ctx) {
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
	ygocdb.OnFullMatchGroup([]string{"/ys", "随机一卡"}, checkUpdate).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		data := drawCard()
		pic, err := drawimage(data)
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return
		}
		ctx.SendChain(message.ImageBytes(pic))
	})
	ygocdb.OnRegex(`^查(日版|英版|简中)卡盒\s*(.+)`).SetBlock(true).Handle(func(ctx *zero.Ctx) {
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
			ctx.SendChain(message.Text("暂不支持,TDB"))
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

	ygocdb.OnRegex(`(取消)?订阅每日随机一卡`, zero.UserOrGrpAdmin, getDB).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		lock.Lock()
		defer lock.Unlock()
		gid := ctx.Event.GroupID
		if gid == 0 {
			gid = -ctx.Event.UserID
		}
		newStatus := ctx.State["regex_matched"].([]string)[1]
		// 获取用户状态
		status := database.getStatus(gid)
		if status {
			if newStatus == "取消" {
				err := database.delStatus(gid)
				if err != nil {
					ctx.SendChain(message.Text("取消订阅失败~\n", serviceErr, err))
					return
				}
				ctx.SendChain(message.Text("已取消订阅~"))
				return
			}
			ctx.SendChain(message.Text("订阅成功~以后每天12点将会自动分享一张卡片~"))
			return
		}
		if newStatus == "取消" {
			ctx.SendChain(message.Text("[ygocdb]尚未订阅过该服务,无需取消"))
			return
		}
		err := database.addStatus(gid)
		if err != nil {
			ctx.SendChain(message.Text("订阅失败~\n", serviceErr, err))
			return
		}
		ctx.SendChain(message.Text("订阅成功~以后每天12点将会自动分享一张卡片~"))
	})
	ygocdb.OnFullMatch("分享卡片", getDB, checkUpdate).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		infos, err := database.findAll()
		if err != nil {
			logrus.Warningf("%s %s", serviceErr, err.Error())
			return
		}
		if len(infos) == 0 {
			return
		}
		for _, info := range infos {
			data := drawCard()
			pic, err := drawimage(data)
			if err != nil {
				logrus.Errorf("%s 发送给群(%d)图片失败。%s", serviceErr, info.GID, err.Error())
				continue
			}
			if info.GID < 0 {
				ctx.SendPrivateMessage(-info.GID, message.Text("今日分享卡片:\n(取消指令:取消订阅每日随机一卡)"))
				ctx.SendPrivateMessage(-info.GID, message.ImageBytes(pic))
			} else if info.GID > 0 {
				ctx.SendGroupMessage(info.GID, message.Text("今日分享卡片:\n(取消指令:取消订阅每日随机一卡)"))
				ctx.SendGroupMessage(info.GID, message.ImageBytes(pic))
			}
			time.Sleep(1 * time.Second)
		}
	})
}

// GetCardInfo 获取卡片信息
func GetCardInfo(cardName string) (cardData []cardInfo, err error) {
	data, err := web.GetData(api + url.QueryEscape(cardName))
	if err != nil {
		return
	}
	var result searchResult
	err = json.Unmarshal(data, &result)
	if err != nil {
		return
	}
	cardData = result.Result
	return
}

func Cardtext(card cardInfo) string {
	var cardtext []string
	name := "C N卡名: " + card.CnName
	cardtext = append(cardtext, name)
	if card.NwbbsN != "" {
		name = "N W卡名: " + card.NwbbsN
		cardtext = append(cardtext, name)
	}
	if card.CnocgN != "" {
		name = "简中卡名: " + card.CnocgN
		cardtext = append(cardtext, name)
	}
	if card.NwbbsN != "" {
		name = "M D卡名: " + card.MdName
		cardtext = append(cardtext, name)
	}
	if card.JpName != "" {
		name = "日本卡名:"
		if card.JpRuby != "" && card.JpName != card.JpRuby {
			name += "\n    " + card.JpRuby
		}
		name += "\n    " + card.JpName
		cardtext = append(cardtext, name)
	}
	if card.EnName != "" {
		name = "英文卡名:\n    " + card.EnName
		cardtext = append(cardtext, name)
	}
	if card.ScName != "" {
		name = "其他译名: " + card.ScName
		cardtext = append(cardtext, name)
	}
	cardtext = append(cardtext, "卡片密码："+strconv.Itoa(card.ID))
	cardtext = append(cardtext, card.Text.Types)
	if card.Text.Pdesc != "" {
		cardtext = append(cardtext, "[灵摆效果]\n"+card.Text.Pdesc)
		if strings.Contains(card.Text.Types, "效果") {
			cardtext = append(cardtext, "[怪兽效果]")
		} else {
			cardtext = append(cardtext, "[怪兽描述]")
		}
	}
	cardtext = append(cardtext, card.Text.Desc)
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

func drawCard(index ...int) cardInfo {
	data := cardInfo{}
	pageMax := len(cradList)
	if pageMax > 0 {
		data = localJSONData[cradList[rand.Intn(pageMax)]]
	}
	i := 0
	if len(index) > 0 {
		i = index[0]
	}
	if i > 10 {
		return data
	}
	if data.ID == 0 {
		i++
		data = drawCard(i)
	}
	return data
}

// getStatus 获取状态
func (cdb *ygoDB) getStatus(gid int64) bool {
	cdb.Lock()
	defer cdb.Unlock()
	return cdb.db.CanFind("subscribe", "WHERE GID = ?", gid)
}

// delStatus 删除状态
func (cdb *ygoDB) delStatus(gid int64) error {
	cdb.Lock()
	defer cdb.Unlock()
	return cdb.db.Del("subscribe", "WHERE GID = ?", gid)
}

// addStatus 添加状态
func (cdb *ygoDB) addStatus(gid int64) error {
	cdb.Lock()
	defer cdb.Unlock()
	return cdb.db.Insert("subscribe", &subscribe{GID: gid})
}

// findAll 查询所有库信息
func (sdb *ygoDB) findAll() (dbInfos []*subscribe, err error) {
	sdb.Lock()
	defer sdb.Unlock()
	return sql.FindAll[subscribe](&sdb.db, "subscribe", "")
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
