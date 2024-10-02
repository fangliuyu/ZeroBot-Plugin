// Package ygosem 基于ygosem的插件功能
package ygosem

import (
	"errors"
	"math/rand"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FloatTech/floatbox/web"
	sql "github.com/FloatTech/sqlite"
	"github.com/wdvxdr1123/ZeroBot/utils/helper"
)

const (
	semurl = "https://www.ygo-sem.cn/"
	ua     = "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Mobile Safari/537.36"
)

type carddb struct {
	db *sql.Sqlite
	sync.RWMutex
}

type roominfo struct {
	GroupID     int64  // 群ID
	GameCard    string // 答案
	Gametype    string // 类型
	LastTime    int64  // 距离上次回答时间
	Worry       int    // 错误次数
	TickCount   int    // 提示次数
	AnswerCount int    // 问答次数
}

type punish struct {
	GroupID  int64 // 群ID
	LastTime int64 // 时间
	Value    int   // 惩罚值
}

type gameCardInfo struct {
	Name    string // 卡名
	ID      string // 卡密
	Type    string // 种类
	Race    string // 种族
	Attr    string // 属性
	Level   string // 等级
	Atk     string // 攻击力
	Def     string // 防御力
	Depict  string // 效果
	PicFile string // 图片文件
}

var (
	mu        sync.RWMutex
	carddatas = &carddb{
		db: &sql.Sqlite{},
	}
)

// web获取卡片信息
func getSemData() (cardData gameCardInfo, err error) {
	url := "https://www.ygo-sem.cn/Cards/Default.aspx"
	// 请求html页面
	body, err := web.RequestDataWith(web.NewDefaultClient(), url, "GET", semurl, ua, nil)
	if err != nil {
		return
	}
	// 获取卡牌数量
	listmax := regexp.MustCompile(`条 共:\s*(?s:(.*?))\s*条</span>`).FindAllStringSubmatch(helper.BytesToString(body), -1)
	if len(listmax) == 0 {
		err = errors.New("数据存在错误: 无法获取当前卡池数量")
		return
	}
	maxnumber, _ := strconv.Atoi(listmax[0][1])
	drawCard := strconv.Itoa(rand.Intn(maxnumber + 1))
	url = "https://www.ygo-sem.cn/Cards/S.aspx?q=" + drawCard
	// 获取卡片信息
	body, err = web.RequestDataWith(web.NewDefaultClient(), url, "GET", semurl, ua, nil)
	if err != nil {
		return
	}
	// 获取卡面信息
	cardData = getCarddata(helper.BytesToString(body))
	if reflect.DeepEqual(cardData, gameCardInfo{}) {
		err = errors.New("数据存在错误: 无法获取卡片信息")
		return
	}
	cardData.Depict = strings.ReplaceAll(cardData.Depict, "\n\r", "")
	cardData.Depict = strings.ReplaceAll(cardData.Depict, "\n", "")
	cardData.Depict = strings.ReplaceAll(cardData.Depict, " ", "")
	field := regexpmatch(`「(?s:(.*?))」`, cardData.Depict)
	if len(field) != 0 {
		for i := 0; i < len(field); i++ {
			cardData.Depict = strings.ReplaceAll(cardData.Depict, field[i][0], "「xxx」")
		}
	}
	err = carddatas.insert(cardData)
	if err != nil {
		return
	}
	// 获取卡图连接
	picHref := regexp.MustCompile(`picsCN(/\d+/\d+).jpg`).FindAllStringSubmatch(helper.BytesToString(body), -1)
	if len(picHref) == 0 {
		err = errors.New("数据存在错误: 无法获取卡图信息")
		return
	}
	url = "https://www.ygo-sem.cn/yugioh/larg/" + picHref[0][1] + ".jpg"
	picByte, downerr := web.RequestDataWith(web.NewDefaultClient(), url, "GET", semurl, ua, nil)
	if downerr != nil {
		err = downerr
		return
	}
	mu.Lock()
	defer mu.Unlock()
	cardData.PicFile = cardData.Name + ".jpg"
	err = os.WriteFile(cachePath+cardData.PicFile, picByte, 0644)
	return
}

// 正则筛选数据
func regexpmatch(rule, str string) [][]string {
	return regexp.MustCompile(rule).FindAllStringSubmatch(str, -1)
}

// 正则返回第n组的数据
func regexpmatchByRaw(rule, str string, n int) []string {
	reg := regexpmatch(rule, str)
	if reg == nil {
		return nil
	}
	if n > len(reg) {
		return reg[len(reg)-1]
	}
	return reg[n]
}

// 正则返回第0组的数据
func regexpmatchByZero(rule, str string) []string {
	return regexpmatchByRaw(rule, str, 0)
}

// 获取卡面信息
func getCarddata(body string) (cardata gameCardInfo) {
	// 获取卡名
	cardName := regexpmatchByZero(`<b>中文名</b> </span>&nbsp;<span class="item_box_value">\s*(.*)</span>\s*</div>`, body)
	if len(cardName) == 0 {
		return
	}
	cardata.Name = cardName[1]
	// 获取卡密
	cardID := regexpmatchByZero(`<b>卡片密码</b> </span>&nbsp;<span class="item_box_value">\s*(.*)\s*</span>`, body)
	cardata.ID = cardID[1]
	// 种类
	cardType := regexpmatchByZero(`<b>卡片种类</b> </span>&nbsp;<span class="item_box_value" id="dCnType">\s*(.*?)\s*</span>\s*<span`, body)
	cardata.Type = cardType[1]
	if strings.Contains(cardType[1], "怪兽") {
		// 种族
		cardRace := regexpmatchByZero(`<span id="dCnRace" class="item_box_value">\s*(.*)\s*</span>\s*<span id="dEnRace"`, body)
		cardata.Race = cardRace[1]
		// 属性
		cardAttr := regexpmatchByZero(`<b>属性</b> </span>&nbsp;<span class="item_box_value" id="attr">\s*(.*)\s*</span>`, body)
		cardata.Attr = cardAttr[1]
		/*星数*/
		switch {
		case strings.Contains(cardType[1], "连接"):
			cardLevel := regexpmatchByZero(`<span class="item_box_value">(LINK.*)</span>`, body)
			cardata.Level = cardLevel[1]
		default:
			cardLevel := regexpmatchByZero(`<b>星数/阶级</b> </span><span class=\"item_box_value\">\s*(.*)\s*</span>`, body)
			cardata.Level = cardLevel[1]
			// 守备力
			cardDef := regexpmatchByZero(`<b>DEF</b></span>\s*&nbsp;<span class="item_box_value">\s*(\d+|\?|？)\s*</span>\s*</div>`, body)
			cardata.Def = cardDef[1]
		}
		// 攻击力
		cardAtk := regexpmatchByZero(`<b>ATK</b> </span>&nbsp;<span class=\"item_box_value\">\s*(\d+|\?|？)\s*</span>`, body)
		cardata.Atk = cardAtk[1]
	}
	/*效果*/
	cardDepict := regexpmatchByZero(`<div class="item_box_text" id="cardDepict">\s*(?s:(.*?))\s*</div>`, body)
	cardata.Depict = cardDepict[1]
	return
}

// 保存卡片信息
func (sql *carddb) insert(dbInfo gameCardInfo) error {
	sql.Lock()
	defer sql.Unlock()
	err := sql.db.Create("cards", &gameCardInfo{})
	if err == nil {
		return sql.db.Insert("cards", &dbInfo)
	}
	return err
}

// 保存卡片信息
func (sql *carddb) reInsertPic(dbInfo gameCardInfo) (gameCardInfo, error) {
	url := "https://www.ygo-sem.cn/Cards/S.aspx?q=" + url.QueryEscape(dbInfo.Name)
	// 请求html页面
	body, err := web.RequestDataWith(web.NewDefaultClient(), url, "GET", semurl, ua, nil)
	if err != nil {
		return dbInfo, err
	}
	// 获取卡图连接
	cardpic := regexpmatchByZero(`picsCN(/\d+/\d+).jpg`, helper.BytesToString(body))
	if len(cardpic) == 0 {
		return dbInfo, errors.New("getPic正则匹配失败")
	}
	url = "https://www.ygo-sem.cn/yugioh/larg/" + cardpic[1] + ".jpg"
	picByte, err := web.RequestDataWith(web.NewDefaultClient(), url, "GET", semurl, ua, nil)
	if err != nil {
		return dbInfo, err
	}
	mu.Lock()
	dbInfo.PicFile = dbInfo.Name + ".jpg"
	err = os.WriteFile(cachePath+dbInfo.PicFile, picByte, 0644)
	mu.Unlock()
	if err != nil {
		return dbInfo, err
	}
	sql.Lock()
	defer sql.Unlock()
	err = sql.db.Create("cards", &gameCardInfo{})
	if err == nil {
		return dbInfo, sql.db.Insert("cards", &dbInfo)
	}
	return dbInfo, err
}

// 随机抽取卡片
func (sql *carddb) load(name string) (dbInfo gameCardInfo, err error) {
	sql.RLock()
	defer sql.RUnlock()
	err = sql.db.Create("cards", &gameCardInfo{})
	if err == nil {
		err = sql.db.Find("cards", &dbInfo, "where Name = '"+name+"'")
	}
	return
}

// 随机抽取卡片
func (sql *carddb) pick() (dbInfo gameCardInfo, err error) {
	sql.RLock()
	defer sql.RUnlock()
	err = sql.db.Create("cards", &gameCardInfo{})
	if err == nil {
		err = sql.db.Pick("cards", &dbInfo)
	}
	return
}

// 加载惩罚值
func (sql *carddb) loadpunish(gid int64, i int) error {
	sql.Lock()
	defer sql.Unlock()
	err := sql.db.Create("punish", &punish{})
	if err == nil {
		var groupInfo punish
		_ = sql.db.Find("punish", &groupInfo, "where GroupID = "+strconv.FormatInt(gid, 10))
		groupInfo.GroupID = gid
		groupInfo.LastTime = time.Now().Unix()
		groupInfo.Value += i
		return sql.db.Insert("punish", &groupInfo)
	}
	return err
}

// 判断惩罚值
func (sql *carddb) checkGroup(gid int64) (float64, bool) {
	sql.Lock()
	defer sql.Unlock()
	err := sql.db.Create("punish", &punish{})
	if err != nil {
		return 0, true
	}
	var groupInfo punish
	_ = sql.db.Find("punish", &groupInfo, "where GroupID = "+strconv.FormatInt(gid, 10))
	if groupInfo.LastTime > 0 {
		subTime := time.Since(time.Unix(groupInfo.LastTime, 0)).Minutes()
		if subTime >= 30 {
			groupInfo.LastTime = time.Now().Unix()
			groupInfo.Value = 0
			_ = sql.db.Insert("punish", &groupInfo)
			return subTime, true
		} else if groupInfo.Value >= 30 {
			return subTime, false
		}
	}
	return 0, true
}

// 加载房间信息
func (sql *carddb) loadRoomInfo(gid int64) (groupInfo roominfo) {
	sql.Lock()
	defer sql.Unlock()
	err := sql.db.Create("rooms", &roominfo{})
	if err != nil {
		return
	}
	err = sql.db.Find("rooms", &groupInfo, "where GroupID = "+strconv.FormatInt(gid, 10))
	if err == nil {
		_ = sql.db.Del("rooms", "where GroupID = "+strconv.FormatInt(gid, 10))
	}
	return
}

// 保存房间信息
func (sql *carddb) saveRoomInfo(info roominfo) error {
	sql.Lock()
	defer sql.Unlock()
	err := sql.db.Create("rooms", &roominfo{})
	if err == nil {
		return sql.db.Insert("rooms", &info)
	}
	return err
}
