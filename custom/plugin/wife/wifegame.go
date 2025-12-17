// Package wife 抽老婆
package wife

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FloatTech/floatbox/file"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"

	zbmath "github.com/FloatTech/floatbox/math"
	"github.com/FloatTech/imgfactory"
	trshttp "github.com/fumiama/terasu/http"
)

var (
	mu       sync.RWMutex
	sizeList = []int{0, 3, 5, 8}
	enguess  = control.Register("wifegame", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Help:             "- 猜老婆",
		Brief:            "从老婆库猜老婆",
	}).ApplySingle(ctxext.NewGroupSingle("已经有正在进行的游戏..."))
	msg = "\n————————\nTips: 老婆库扩展计划\n要求：图片尽可能纯角色立绘\n指令:\n扩容老婆库 出处 角色名 + 图片"
)

func init() {
	_ = os.MkdirAll(engine.DataFolder()+"temp", 0755)
	enguess.OnFullMatch("同步本地老婆库", zero.AdminPermission).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		err := updateNativeCards()
		if err != nil {
			ctx.SendChain(message.Text("同步失败:\n", err))
			return
		}
		ctx.SendChain(message.Text("同步成功!"))
	})
	enguess.OnPrefix("扩容老婆库", zero.MustProvidePicture).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		msgList := strings.Split(ctx.ExtractPlainText(), " ")
		if len(msgList) < 3 {
			ctx.SendChain(message.Text("请按照格式发送:\n扩容老婆库 + 出处 + 角色名 + 图片"))
			return
		}
		pics, ok := ctx.State["image_url"].([]string)
		if !ok {
			ctx.SendChain(message.Text("ERROR: 未获取到图片链接"))
			return
		}
		work := msgList[1]
		name := strings.Join(msgList[2:], " ")
		if work == "" || name == "" {
			ctx.SendChain(message.Text("ERROR: 出处或角色名不能为空"))
			return
		}
		fileName := "[" + work + "]" + name + " - P"
		path := engine.DataFolder() + "temp" + "/"
		if file.IsNotExist(path) {
			ctx.SendChain(message.Text("ERROR: 图库文件夹不存在"))
			return
		}
		ctx.SendChain(message.Text("好的，已收到"))
		mu.Lock()
		files, err := os.ReadDir(path)
		if err != nil {
			return
		}
		// 如果本地列表为空
		if len(files) != 0 {
			// 按名称从小到大排列
			sort.Slice(files, func(i, j int) bool {
				return files[i].Name() < files[j].Name()
			})
			i := -1
			for _, file := range files {
				if strings.HasPrefix(file.Name(), fileName) {
					name, _, _ := strings.Cut(file.Name(), ".")
					name = strings.TrimPrefix(name, fileName)
					i, _ = strconv.Atoi(name)
					if i < 0 {
						i = -1
					}
					continue
				}
			}
			fileName += strconv.Itoa(i + 1)
		} else {
			fileName += "0"
		}
		fileName += ".png"
		mu.Unlock()

		err = downloadToFile(pics[0], file.BOTPATH+"/"+engine.DataFolder()+"temp/", fileName)
		if err != nil {
			log.Println("[wife]扩容失败:", err)
		}
	})
	enguess.OnFullMatch("猜老婆", getJSON).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		class := 3

		card := cards[rand.Intn(len(cards))]
		pic, err := engine.GetLazyData("wives/"+card, true)
		if err != nil {
			ctx.SendChain(message.Text("[猜老婆]error:\n", err))
			return
		}
		work, name := card2name(card)
		name = strings.ToLower(name)
		img, _, err := image.Decode(bytes.NewReader(pic))
		if err != nil {
			ctx.SendChain(message.Text("[猜老婆]error:\n", err))
			return
		}
		dst := imgfactory.Size(img, img.Bounds().Dx(), img.Bounds().Dy())
		q, err := mosaic(dst, class)
		if err != nil {
			ctx.SendChain(
				message.Reply(ctx.Event.MessageID),
				message.Text("[猜老婆]图片生成失败:\n", err),
			)
			return
		}
		if id := ctx.SendChain(
			message.ImageBytes(q),
		); id.ID() != 0 {
			ctx.SendChain(message.Text("请回答该二次元角色名字\n以“xxx酱”格式回答\n发送“跳过”结束猜题"))
		}
		var next *zero.FutureEvent
		if ctx.Event.GroupID == 0 {
			next = zero.NewFutureEvent("message", 999, false, zero.RegexRule(`^(·)?[^酱]+酱|^跳过$`), ctx.CheckSession())
		} else {
			next = zero.NewFutureEvent("message", 999, false, zero.RegexRule(`^(·)?[^酱]+酱|^跳过$`), zero.CheckGroup(ctx.Event.GroupID))
		}
		recv, cancel := next.Repeat()
		defer cancel()
		tick := time.NewTimer(105 * time.Second)
		after := time.NewTimer(120 * time.Second)
		for {
			select {
			case <-tick.C:
				ctx.SendChain(message.Text("[猜老婆]你还有15s作答时间"))
			case <-after.C:
				ctx.Send(
					message.ReplyWithMessage(ctx.Event.MessageID,
						message.ImageBytes(pic),
						message.Text("[猜老婆]倒计时结束，游戏结束...\n角色是:\n", name, "\n出自《", work, "》", msg),
					),
				)
				return
			case c := <-recv:
				// tick.Reset(105 * time.Second)
				// after.Reset(120 * time.Second)
				msg := strings.ReplaceAll(c.Event.Message.String(), "酱", "")
				if msg == "" {
					continue
				}
				if msg == "跳过" {
					if msgID := ctx.Send(message.ReplyWithMessage(c.Event.MessageID,
						message.Text("已跳过猜题\n角色是:\n", name, "\n出自《", work, "》\n"),
						message.ImageBytes(pic))); msgID.ID() == 0 {
						ctx.SendChain(message.Text("太棒了,你猜对了!\n图片发送失败,可能被风控\n角色是:\n", name, "\n出自《", work, "》", msg))
					}
					return
				}
				class--
				if strings.Contains(name, strings.ToLower(msg)) {
					if msgID := ctx.Send(message.ReplyWithMessage(c.Event.MessageID,
						message.Text("太棒了,你猜对了!\n角色是:\n", name, "\n出自《", work, "》\n"),
						message.ImageBytes(pic))); msgID.ID() == 0 {
						ctx.SendChain(message.Text("太棒了,你猜对了!\n图片发送失败,可能被风控\n角色是:\n", name, "\n出自《", work, "》", msg))
					}
					return
				}
				if class < 1 {
					if msgID := ctx.Send(message.ReplyWithMessage(c.Event.MessageID,
						message.Text("很遗憾,次数到了,游戏结束!\n角色是:\n", name, "\n出自《", work, "》\n"),
						message.ImageBytes(pic))); msgID.ID() == 0 {
						ctx.SendChain(message.Text("很遗憾,次数到了,游戏结束!\n图片发送失败,可能被风控\n角色是:\n", name, "\n出自《", work, "》", msg))
					}
					return
				}
				q, err = mosaic(dst, class)
				if err != nil {
					ctx.SendChain(
						message.Text("回答错误,你还有", class, "次机会\n请继续作答\n(提示：", work, ")"),
					)
					continue
				}
				msg = ""
				if class == 2 {
					msg = "(提示：" + work + ")\n"
				}
				ctx.SendChain(
					message.Text("回答错误,你还有", class, "次机会\n", msg, "请继续作答(难度降低)\n"),
					message.ImageBytes(q),
				)
				continue
			}
		}
	})
}

func downloadToFile(url, folder, name string) error {
	mu.Lock()
	defer mu.Unlock()
	// resp, err := http.Get(url)
	// if err != nil {
	// 	return "", fmt.Errorf("下载请求失败: %v", err)
	// }
	// defer resp.Body.Close()

	var resp *http.Response
	resp, err := trshttp.Get(url)
	if err != nil {
		fmt.Println("trshttp:", err)
		resp, err = http.Get(url)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败, HTTP状态码: %d", resp.StatusCode)
	}

	if resp.Body == http.NoBody {
		return fmt.Errorf("下载失败, 无内容")
	}

	imagePath := filepath.Join(folder, name)
	contentLength := resp.ContentLength
	var downloaded int64

	out, err := os.Create(imagePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer out.Close()

	// 创建带缓冲的写入器
	buf := make([]byte, 32*1024) // 32KB缓冲区
	writer := io.MultiWriter(out)

	for {
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := writer.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			downloaded += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}

		// 显示进度（仅当知道总大小时）
		if contentLength > 0 {
			fmt.Printf("\rDownloading... %d%%\n", int(downloaded*100/contentLength))
		}
	}

	return err
}

// // 从本地图库随机抽取，规避网络问题
// func lottery() (fileName string, err error) {
// 	path := engine.DataFolder() + "wives" + "/"
// 	if file.IsNotExist(path) {
// 		err = errors.New("图库文件夹不存在,请先发送“抽老婆”扩展图库")
// 		return
// 	}
// 	files, err := os.ReadDir(path)
// 	if err != nil {
// 		return
// 	}
// 	// 如果本地列表为空
// 	if len(files) == 0 {
// 		err = errors.New("本地数据为0,请先发送“抽老婆”扩展图库")
// 		return
// 	}
// 	fileName = randPicture(files)
// 	if fileName == "" {
// 		err = errors.New("抽取图库轮空了,请重试")
// 	}
// 	return
// }

func updateNativeCards() error {
	path := engine.DataFolder() + "wives" + "/"
	if file.IsNotExist(path) {
		return errors.New("图库文件夹不存在,请先发送“抽老婆”扩展图库")
	}
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	// 如果本地列表为空
	if len(files) == 0 {
		return errors.New("本地数据为0,请先发送“抽老婆”扩展图库")
	}
	cards = []string{}
	for _, file := range files {
		if !file.IsDir() {
			add := true
			for _, name := range cards {
				if name == file.Name() {
					add = false
					continue
				}
			}
			if add {
				cards = append(cards, file.Name())
			}
		}
	}
	jsonData, err := json.MarshalIndent(cards, "", "  ")
	if err != nil {
		return err
	}

	file, err := os.Create(engine.DataFolder() + "wife.json")
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(jsonData)
	return err
}

// func randPicture(files []fs.DirEntry) (fileName string) {
// 	if len(files) == 0 {
// 		return
// 	}
// 	rand.Shuffle(len(files), func(i, j int) {
// 		files[i], files[j] = files[j], files[i]
// 	})

// 	for _, f := range files {
// 		if f.IsDir() || strings.HasPrefix(f.Name(), ".") {
// 			continue
// 		}

// 		if ext := filepath.Ext(f.Name()); ext != "" {
// 			return f.Name()
// 		}
// 	}

// 	return
// }

// 马赛克生成
func mosaic(dst *imgfactory.Factory, level int) ([]byte, error) {
	b := dst.Image().Bounds()
	p := imgfactory.NewFactoryBG(dst.W(), dst.H(), color.NRGBA{255, 255, 255, 255})
	markSize := zbmath.Max(b.Max.X, b.Max.Y) * sizeList[level] / 200

	for yOfMarknum := 0; yOfMarknum <= zbmath.Ceil(b.Max.Y, markSize); yOfMarknum++ {
		for xOfMarknum := 0; xOfMarknum <= zbmath.Ceil(b.Max.X, markSize); xOfMarknum++ {
			a := dst.Image().At(xOfMarknum*markSize+markSize/2, yOfMarknum*markSize+markSize/2)
			cc := color.NRGBAModel.Convert(a).(color.NRGBA)
			for y := 0; y < markSize; y++ {
				for x := 0; x < markSize; x++ {
					xOfPic := xOfMarknum*markSize + x
					yOfPic := yOfMarknum*markSize + y
					p.Image().Set(xOfPic, yOfPic, cc)
				}
			}
		}
	}
	return imgfactory.ToBytes(p.Blur(3).Image())
}
