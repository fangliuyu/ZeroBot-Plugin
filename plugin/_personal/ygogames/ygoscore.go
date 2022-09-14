package ygoscore

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	fcext "github.com/FloatTech/floatbox/ctxext"
	"github.com/FloatTech/floatbox/process"
	ctrl "github.com/FloatTech/zbpctrl"
	control "github.com/FloatTech/zbputils/control"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"

	// 数据库
	sql "github.com/FloatTech/sqlite"

	// 图片输出
	"github.com/Coloured-glaze/gg"
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/img/writer"
	"github.com/FloatTech/floatbox/web"
	"github.com/FloatTech/zbputils/img"
	"github.com/FloatTech/zbputils/img/text"
)

type score struct {
	db   *sql.Sqlite
	dbmu sync.RWMutex
}

// 用户数据信息
type userdata struct {
	Uid          string // `Userid`
	UserName     string // `User`
	Count        int    // `签到次数`
	UpdatedAt    string // `签到时间`
	Continuous   int    // `连续签到次数`
	Level        int    // `决斗者等级`
	Signpoints   int    // `签到积分`
	Obtainpoints int    // `获得的积分`
	Lostpoints   int    // `失去的积分`
}

var (
	scoredata = &score{
		db: &sql.Sqlite{},
	}
	levelArray = [...]int{0, 10, 20, 50, 100, 200, 350, 550, 750, 1000, 1200}
	levelrank  = [...]string{"新手", "青铜", "白银", "黄金", "白金Ⅲ", "白金Ⅱ", "白金Ⅰ", "传奇Ⅲ", "传奇Ⅱ", "传奇Ⅰ", "决斗王"}
)

const (
	// 积分基数
	initSCORE = 100
	// SCOREMAX 分数上限定为120
	SCOREMAX = 1200
)

func init() {
	engine := control.Register("ygoscore", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault:  false,
		PrivateDataFolder: "ygoscore",
		Help: "签到系统\n" +
			"-注册决斗者 xxxx\n" +
			"-注销决斗者 @群友\n" +
			"-签到\n" +
			"-/积分\n" +
			"-/记录 @群友 积分值\n" +
			"-/记录 @加分群友 积分值 @减分群友\n",
	})

	getdb := fcext.DoOnceOnSuccess(func(ctx *zero.Ctx) bool {
		scoredata.db.DBPath = engine.DataFolder() + "score.db"
		err := scoredata.db.Open(time.Hour * 24)
		if err != nil {
			ctx.SendChain(message.Text("ERROR:", err))
			return false
		}
		return true
	})

	engine.OnRegex(`^注册决斗者(\s+)?([^\s]+(\s+[^\s]+)*)`, zero.OnlyGroup, getdb).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		gid := strconv.FormatInt(ctx.Event.GroupID, 10)
		uid := strconv.FormatInt(ctx.Event.UserID, 10)
		userinfo, _ := scoredata.checkuser(gid, uid)
		if userinfo.UserName != "" {
			ctx.SendChain(message.Text("你已经注册过了"))
			return
		}
		username := ctx.State["regex_matched"].([]string)[2]
		if strings.Contains(username, "[CQ:face,id=") {
			ctx.SendChain(message.Text("用户名不支持表情包"))
			return
		}
		lenmane := []rune(username)
		if len(lenmane) > 10 {
			ctx.SendChain(message.Text("决斗者昵称不得长于10个字符"))
			return
		}
		ok, err := scoredata.register(gid, uid, username)
		if err != nil {
			ctx.SendChain(message.Text("用户登记失败！请联系bot管理员"))
			return
		}
		if ok {
			ctx.SendChain(message.Text("注册成功"))
			return
		}
	})
	engine.OnRegex(`^注销决斗者\s*?\[CQ:at,qq=(\d+)\].*?`, zero.OnlyGroup, zero.AdminPermission, getdb).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		adduser := ctx.State["regex_matched"].([]string)[1]
		gid := strconv.FormatInt(ctx.Event.GroupID, 10)
		ok, err := scoredata.deleteuser(gid, adduser)
		if err != nil {
			ctx.SendChain(message.Text("用户注销失败！请联系bot管理员"))
			return
		}
		if ok {
			ctx.SendChain(message.Text("注销成功"))
			return
		}
		ctx.SendChain(message.Text("用户没有注册过"))
	})

	engine.OnFullMatchGroup([]string{"签到", "打卡"}, zero.OnlyGroup, getdb).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		gid := strconv.FormatInt(ctx.Event.GroupID, 10)
		uid := strconv.FormatInt(ctx.Event.UserID, 10)
		userinfo, err := scoredata.checkuser(gid, uid)
		if err != nil || userinfo.UserName == "" {
			ctx.SendChain(message.Text("决斗者未注册！\n请输入“注册决斗者 xxx”进行登记(xxx为决斗者昵称)。"))
			return
		}
		now := time.Now()
		today := now.Format("2006/01/02 15:04:05")
		lasttime, _ := time.ParseInLocation("2006/01/02 15:04:05", userinfo.UpdatedAt, time.Local)
		if lasttime.Format("2006/01/02") != now.Format("2006/01/02") {
			userinfo.Count = 0
			if err := scoredata.db.Insert(gid, &userinfo); err != nil {
				ctx.SendChain(message.Text("签到数据更新失败，请再次尝试或者联系bot管理员"))
				return
			}
		}
		// 判断是否已经签到过了
		if userinfo.Count >= 1 || lasttime.Format("2006/01/02") == now.Format("2006/01/02") {
			// 生成积分图片
			data, cl, err := drawimage(&userinfo, 0)
			if err != nil {
				ctx.SendChain(message.Text("[error]", err))
				return
			}
			ctx.SendChain(message.Text("今天已经签到过了"))
			ctx.SendChain(message.ImageBytes(data))
			cl()
			return
		}
		// 更新数据
		add := 1
		subtime := now.Sub(lasttime).Hours()
		if subtime > 48 {
			userinfo.Continuous = 1
		} else {
			userinfo.Continuous += 1
			add = int(math.Min(5, float64(userinfo.Continuous)))
		}
		userinfo.Count += 1
		userinfo.UpdatedAt = today
		if userinfo.Level < SCOREMAX {
			userinfo.Level += 1
		}
		userinfo.Signpoints += add
		if err := scoredata.db.Insert(gid, &userinfo); err != nil {
			ctx.SendChain(message.Text("签到失败，请再次尝试或者联系bot管理员"))
			return
		}
		// 生成签到图片
		data, cl, err := drawimage(&userinfo, add)
		if err != nil {
			ctx.SendChain(message.Text("[error]", err))
			return
		}
		ctx.SendChain(message.ImageBytes(data))
		cl()
	})
	engine.OnFullMatch("/积分", zero.OnlyGroup, getdb).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		gid := strconv.FormatInt(ctx.Event.GroupID, 10)
		uid := strconv.FormatInt(ctx.Event.UserID, 10)
		userinfo, err := scoredata.checkuser(gid, uid)
		if err != nil || userinfo.UserName == "" {
			ctx.SendChain(message.Text("决斗者未注册！\n请输入“注册决斗者 xxx”进行登记(xxx为决斗者昵称)。"))
			return
		}
		// 生成图片
		data, cl, err := drawimage(&userinfo, 0)
		if err != nil {
			ctx.SendChain(message.Text("[error]", err))
			return
		}
		ctx.SendChain(message.ImageBytes(data))
		cl()
	})
	engine.OnRegex(`^\/记录\s*?\[CQ:at,qq=(\d+)\]\s*?(-?\d+)((\s*)?\[CQ:at,qq=(\d+)\])?`, zero.OnlyGroup, zero.AdminPermission).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		adduser := ctx.State["regex_matched"].([]string)[1]
		score, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[2])
		devuser := ctx.State["regex_matched"].([]string)[5]
		gid := strconv.FormatInt(ctx.Event.GroupID, 10)
		adduserinfo, err := scoredata.checkuser(gid, adduser)
		if err != nil {
			scoredata.register(gid, adduser, "")
			adduserinfo, _ = scoredata.checkuser(gid, adduser)
		}
		if score > 0 {
			adduserinfo.Obtainpoints += score
			adduserinfo.Level += int(math.Ceil(float64(score) / 10))
		} else if score < 0 && devuser == "" {
			adduserinfo.Lostpoints -= score
		} else {
			ctx.SendChain(message.Text("积分只能大于0"))
			return
		}
		if err := scoredata.db.Insert(gid, &adduserinfo); err != nil {
			ctx.SendChain(message.Text("数据更新失败，请再次尝试或者联系bot管理员"))
			return
		}
		ctx.SendChain(message.Text("记录成功"))
		process.SleepAbout1sTo2s()
		adduserid, _ := strconv.ParseInt(adduser, 10, 64)
		switch {
		case score > 0:
			ctx.SendChain(message.At(adduserid), message.Text("你获取积分:", score))
		case score < 0:
			ctx.SendChain(message.At(adduserid), message.Text("你失去积分:", score))
		}
		if devuser == "" {
			return
		}
		devuserid, _ := strconv.ParseInt(devuser, 10, 64)
		devuserinfo, err := scoredata.checkuser(gid, devuser)
		if err != nil {
			scoredata.register(gid, devuser, "")
			devuserinfo, _ = scoredata.checkuser(gid, devuser)
		}
		devuserinfo.Lostpoints += score
		if err := scoredata.db.Insert(gid, &devuserinfo); err != nil {
			ctx.SendChain(message.Text("数据更新失败，请联系bot管理员"))
			return
		}
		ctx.SendChain(message.At(devuserid), message.Text("你失去积分:", score))
	})
}

func (p *score) register(gid, uid, username string) (ok bool, err error) {
	p.dbmu.Lock()
	defer p.dbmu.Unlock()
	if err := p.db.Create(gid, &userdata{}); err != nil {
		return false, err
	}
	if err := p.db.Find(gid, &userdata{}, "where userName = "+username); err == nil {
		return false, nil
	}
	var userinfo userdata
	if err := p.db.Find(gid, &userinfo, "where uid = "+uid); err == nil {
		if userinfo.UserName != "" {
			return false, nil
		}
	}
	userinfo = userdata{
		Uid:       uid,
		UserName:  username,
		UpdatedAt: "2006/01/02 15:04:05",
	}
	if err := p.db.Insert(gid, &userinfo); err != nil {
		return false, err
	}
	return true, nil
}
func (p *score) checkuser(gid, uid string) (userinfo userdata, err error) {
	p.dbmu.Lock()
	defer p.dbmu.Unlock()
	if err = p.db.Create(gid, &userdata{}); err != nil {
		return
	}
	err = p.db.Find(gid, &userinfo, "where uid = "+uid)
	return
}

// 绘制图片
func drawimage(userinfo *userdata, Signpoints int) (data []byte, cl func(), err error) {
	/***********获取头像***********/
	backX := 500
	backY := 500
	data, err = web.GetData("http://q4.qlogo.cn/g?b=qq&nk=" + userinfo.Uid + "&s=640&cache=0")
	if err != nil {
		return
	}
	back, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return
	}
	back = img.Size(back, backX, backY).Im
	/***********设置图片的大小和底色***********/
	canvas := gg.NewContext(1500, 650)
	canvas.SetRGB(1, 1, 1)
	canvas.Clear()

	/***********放置头像***********/
	canvas.DrawImage(back, 0, 0)

	/***********写入用户信息***********/
	fontSize := 50.0
	_, err = file.GetLazyData(text.BoldFontFile, false)
	if err != nil {
		return
	}
	if err = canvas.LoadFontFace(text.BoldFontFile, fontSize); err != nil {
		return
	}
	canvas.SetRGB(0, 0, 0)
	length, h := canvas.MeasureString(userinfo.Uid)
	// 用户名字和QQ号
	n, _ := canvas.MeasureString(userinfo.UserName)
	canvas.DrawString(userinfo.UserName, 550, 130-h)
	canvas.DrawRoundedRectangle(600+n-length*0.1, 130-h*2.5, length*1.2, h*2, fontSize*0.2)
	canvas.SetRGB255(221, 221, 221)
	canvas.Fill()
	canvas.SetRGB(0, 0, 0)
	canvas.DrawString(userinfo.Uid, 600+n, 130-h)
	// 填如签到数据
	level := getLevel(userinfo.Level)
	if Signpoints == 0 {
		canvas.DrawString(fmt.Sprintf("决斗者等级：LV%d", level), 550, 240-h)
		canvas.DrawString("等级阶段: "+levelrank[level], 1030, 240-h)
		canvas.DrawString(fmt.Sprintf("已连续签到 %d 天", userinfo.Continuous), 550, 320-h)
	} else {
		if userinfo.Level < SCOREMAX {
			canvas.DrawString(fmt.Sprintf("经验 +1,积分 +%d", Signpoints), 550, 240-h)
		} else {
			canvas.DrawString(fmt.Sprintf("签到积分 + %d", Signpoints), 550, 240-h)
		}
		canvas.DrawString(fmt.Sprintf("决斗者等级：LV%d", level), 1000, 240-h)
		canvas.DrawString(fmt.Sprintf("已连续签到 %d 天", userinfo.Continuous), 550, 320-h)
	}
	// 绘制等级进度条
	canvas.DrawRectangle(550, 350-h, 900, 80)
	canvas.SetRGB255(150, 150, 150)
	canvas.Fill()
	var nextLevelScore int
	if level < 10 {
		nextLevelScore = levelArray[level+1]
	} else {
		nextLevelScore = SCOREMAX
	}
	canvas.SetRGB255(0, 0, 0)
	canvas.DrawRectangle(550, 350-h, 900*float64(userinfo.Level)/float64(nextLevelScore), 80)
	canvas.SetRGB255(102, 102, 102)
	canvas.Fill()
	canvas.DrawString(fmt.Sprintf("%d/%d", userinfo.Level, nextLevelScore), 1250, 320-h)
	// 更新时间
	canvas.SetRGB(0, 0, 0)
	canvas.DrawString("更新日期："+userinfo.UpdatedAt[:10], 900, 660-h)
	// 积分详情
	canvas.DrawString(fmt.Sprintf("基础积分：%d", initSCORE), 10, 590-h)
	canvas.DrawString(fmt.Sprintf("签到积分：%d", userinfo.Signpoints), 400, 590-h)
	canvas.DrawString(fmt.Sprintf("获取积分：%d", userinfo.Obtainpoints), 10, 660-h)
	canvas.DrawString(fmt.Sprintf("已用积分：%d", userinfo.Lostpoints), 400, 660-h)
	if err = canvas.LoadFontFace(text.BoldFontFile, fontSize*1.5); err != nil {
		return
	}
	score := initSCORE + userinfo.Signpoints + userinfo.Obtainpoints - userinfo.Lostpoints
	canvas.DrawString(fmt.Sprintf("当前总积分：%d", score), 800, 550-h)
	// 生成图片
	data, cl = writer.ToBytes(canvas.Image())
	return
}

func (p *score) deleteuser(gid, uid string) (ok bool, err error) {
	p.dbmu.Lock()
	defer p.dbmu.Unlock()
	if err := p.db.Find(gid, &userdata{}, "where Uid = "+uid); err != nil {
		return false, nil
	}
	if err := p.db.Del(gid, "where Uid = "+uid); err != nil {
		return false, err
	}
	return true, nil
}

func getLevel(count int) int {
	for k, v := range levelArray {
		if count == v {
			return k
		} else if count < v {
			return k - 1
		}
	}
	return -1
}
