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
	"time"

	"archive/zip"
	"encoding/json"

	"github.com/PuerkitoBio/goquery"

	"github.com/FloatTech/floatbox/binary"
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/math"
	"github.com/FloatTech/floatbox/web"
	"github.com/FloatTech/imgfactory"
	ctrl "github.com/FloatTech/zbpctrl"
	control "github.com/FloatTech/zbputils/control"
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

type box_info struct {
	Time   string
	Number string
	Name   string
	Trade  string
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
	lastVersion   = "123"
	lastTime      int
	localJsonData = make(map[string]cardInfo)
	cradList      []string
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
			os.WriteFile(verFile, fileData, 0644)
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
			lastTime = time.Now().Day()
			lastVersion = version
			fileData := binary.StringToBytes(strconv.Itoa(lastTime) + "\n" + lastVersion)
			os.WriteFile(verFile, fileData, 0644)
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
			ctx.SendChain(message.At(ctx.Event.UserID), message.Text("更新成功"))
			return
		}
		ctx.SendChain(message.At(ctx.Event.UserID), message.Text("没发现更新内容"))

	})
	en.OnFullMatchGroup([]string{"分享卡片", "/ys"}, func(ctx *zero.Ctx) bool {
		if time.Now().Day() == lastTime {
			return true
		}
		data, err := web.GetData("https://ygocdb.com/api/v0/cards.zip.md5?callback=gu")
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
		}
		version := binary.BytesToString(data)
		if version != lastVersion {
			lastTime = time.Now().Day()
			lastVersion = version
			fileData := binary.StringToBytes(strconv.Itoa(lastTime) + "\n" + lastVersion)
			os.WriteFile(verFile, fileData, 0644)
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
		boxList := make([]box_info, 0, 256)
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
			doc.Find(".t_body").First().Find(".t_row").Each(func(i int, contentSelection *goquery.Selection) {
				info := box_info{
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
			doc.Find(".t_body").First().Find(".t_row").Each(func(i int, contentSelection *goquery.Selection) {
				info := box_info{
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
			// doc.Find(".t_row").Each(func(i int, contentSelection *goquery.Selection) {
			// 	info := box_info{
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
	err = json.Unmarshal(data, &localJsonData)
	if err != nil {
		return err
	}
	cradList = []string{}
	for key := range localJsonData {
		cradList = append(cradList, key)
	}
	return nil
}

func drawCard() cardInfo {
	data := cardInfo{}
	max := len(cradList)
	if max > 0 {
		data = localJsonData[cradList[rand.Intn(max)]]
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

	h := math.Max(picy, basePicPy)
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
