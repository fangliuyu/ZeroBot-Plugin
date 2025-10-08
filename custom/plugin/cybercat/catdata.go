// Package cybercat 云养猫
package cybercat

import (
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	fcext "github.com/FloatTech/floatbox/ctxext"
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/web"
	sql "github.com/FloatTech/sqlite"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

const apiURL = "https://api.thecatapi.com/v1/images/"

var typeEN2ZH = map[string]string{
	"Abyssinian": "阿比西尼亚猫", "Aegean": "爱琴猫", "American Bobtail": "美国短尾猫", "American Curl": "美国卷耳猫", "American Shorthairs": "美洲短毛猫", "American Wirehair": "美国硬毛猫",
	"Arabian Mau": "美英猫", "Australian Mist": "澳大利亚雾猫", "Balinese": "巴厘岛猫", "Bambino": "班比诺猫", "Bengal": "孟加拉虎", "Birman": "比尔曼猫", "Bombay": "孟买猫", "British Longhair": "英国长毛猫",
	"British Shorthair": "英国短毛猫", "Burmese": "缅甸猫", "Burmilla": "博美拉猫", "California Spangled": "加州闪亮猫", "Chantilly-Tiffany": "查达利/蒂法尼猫", "Chartreux": "夏特鲁斯猫", "Chausie": "非洲狮子猫",
	"Cheetoh": "奇多猫", "Colorpoint Shorthair": "重点色短毛猫", "Cornish Rex": "康沃尔-雷克斯猫", "Cymric": "威尔士猫", "Cyprus": "塞浦路斯猫", "Devon Rex": "德文狸猫", "Donskoy": "顿斯科伊猫", "Dragon Li": "中国狸花猫",
	"Egyptian Mau": "埃及猫", "European Burmese": "欧洲缅甸猫", "Exotic Shorthair": "异国短毛猫", "Havana Brown": "哈瓦那褐猫", "Himalayan": "喜马拉雅猫", "Japanese Bobtail": "日本短尾猫", "Javanese": "爪哇猫",
	"Khao Manee": "泰国御猫", "Korat": "呵叻猫", "Kurilian": "千岛短尾猫", "LaPerm": "拉邦猫", "Maine Coon": "缅因猫", "Malayan": "马来猫", "Manx": "马恩岛猫", "Munchkin": "曼基康猫", "Nebelung": "内华达猫",
	"Norwegian Forest Cat": "挪威森林猫", "Ocicat": "欧西猫", "Oriental Shorthair": "东方短毛猫", "Persian": "波斯猫", "Pixie-bob": "北美洲短毛猫", "Ragamuffin": "褴褛猫", "Ragdoll": "布偶猫",
	"Russian Blue": "俄罗斯蓝猫", "Savannah": "沙凡那猫", "Scottish Fold": "苏格兰折耳猫", "Selkirk Rex": "塞尔凯克卷毛猫", "Siamese": "暹罗猫", "Siberian": "西伯利亚猫", "Singapura": "新加坡猫", "Snowshoe": "雪鞋猫",
	"Somali": "索马里猫", "Sphynx": "斯芬克斯猫", "Tonkinese": "东京猫", "Toyger": "玩具虎猫", "Turkish Angora": "土耳其安哥拉猫",
	"Turkish Van": "土耳其梵猫", "York Chocolate": "约克巧克力猫", "Cymic": "金力克长毛猫"}

var typeZH2Breeds = map[string]string{
	"阿比西尼亚猫": "abys", "爱琴猫": "aege", "美国短尾猫": "abob", "美国卷耳猫": "acur", "美洲短毛猫": "asho", "美国硬毛猫": "awir", "美英猫": "amau", "澳大利亚雾猫": "amis", "巴厘岛猫": "bali",
	"班比诺猫": "bamb", "孟加拉虎": "beng", "比尔曼猫": "birm", "孟买猫": "bomb", "英国长毛猫": "bslo", "英国短毛猫": "bsho", "缅甸猫": "bure", "博美拉猫": "buri", "加州闪亮猫": "cspa",
	"查达利/蒂法尼猫": "ctif", "夏特鲁斯猫": "char", "非洲狮子猫": "chau", "奇多猫": "chee", "重点色短毛猫": "csho", "康沃尔-雷克斯猫": "crex", "威尔士猫": "cymr", "塞浦路斯猫": "cypr",
	"德文狸猫": "drex", "顿斯科伊猫": "dons", "中国狸花猫": "lihu", "埃及猫": "emau", "欧洲缅甸猫": "ebur", "异国短毛猫": "esho", "哈瓦那褐猫": "hbro", "喜马拉雅猫": "hima", "日本短尾猫": "jbob",
	"爪哇猫": "java", "泰国御猫": "khao", "呵叻猫": "kora", "千岛短尾猫": "kuri", "拉邦猫": "lape", "缅因猫": "mcoo", "马来猫": "mala", "马恩岛猫": "manx", "曼基康猫": "munc", "内华达猫": "nebe",
	"挪威森林猫": "norw", "欧西猫": "ocic", "东方短毛猫": "orie", "波斯猫": "pers", "北美洲短毛猫": "pixi", "褴褛猫": "raga", "布偶猫": "ragd", "俄罗斯蓝猫": "rblu", "沙凡那猫": "sava",
	"苏格兰折耳猫": "sfol", "塞尔凯克卷毛猫": "srex", "暹罗猫": "siam", "西伯利亚猫": "sibe", "新加坡猫": "sing", "雪鞋猫": "snow", "索马里猫": "soma", "斯芬克斯猫": "sphy", "东京猫": "tonk",
	"玩具虎猫": "toyg", "土耳其安哥拉猫": "tang", "土耳其梵猫": "tvvan", "约克巧克力猫": "ycho", "金力克长毛猫": "cymi"}

type catdb struct {
	db sql.Sqlite
	sync.RWMutex
}

type warehouse struct {
	User     int64   // 主人
	Food     float64 // 食物数量
	BuyTime  int     // 购买次数
	LastTime int64   // 道具使用时间
	Rua      int     // rua猫次数
	Rrop1    int     // 道具1 - 逗猫棒
	Rrop2    int     // 道具2
	Rrop3    int     // 道具3
	Rrop4    int     // 道具4
	Rrop5    int     // 道具5
	Rrop6    int     // 道具6
	Rrop7    int     // 道具7
	Rrop8    int     // 道具8
}

type catInfo struct {
	User     int64  // 主人
	LastTime int64  // 更新时间
	Name     string // 喵喵名称
	Type     string // 品种
	Breed    int    // 升阶级数

	Mood       int     // 心情
	Satiety    float64 // 饱食度
	Weight     float64 // 体重
	Experience int     // 经验值

	SubTime  float64 // 上次喂食间隔
	WorkTime int64   // 修炼开始时间
	Work     float64 // 修炼时间

	ArenaTime int64 // 上次PK时间

	Picurl string // 猫猫图片
}

var (
	dbpath  = "data/cybercat/catdata.db"
	catdata = &catdb{db: sql.New(dbpath)}
	engine  = control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "云养猫",
		Help: "- 吸猫\n(随机返回一只猫)\n- 吸xxx猫 (吸指定猫种的猫)\n- 买猫\n- 买xxx猫\n- 买猫粮\n- 买n袋猫粮\n- 喂猫\n- 喂猫n斤猫粮\n" +
			"- 猫猫修炼[1-9]小时\n- 猫猫取消修炼\n- 买逗猫棒\n- 买n支逗猫棒\n- 撸猫\n- 猫猫状态\n- 猫猫突破\n- 猫猫改名叫xxx\n" +
			"- 猫猫pk@对方QQ\n- 猫猫排行榜\n" +
			"\n---------注意事项---------" +
			"\n1.第一只猫免费送,再次购买需100\n" +
			"\n2.科学养猪(划去)\n猫猫体重超过25kg会被胖死,低于4kg会瘦死" +
			"\n3.一袋猫粮有五斤猫粮" +
			"\n4.猫猫心情影响修行" +
			"\n5.越重的猫猫饭量越大" +
			"\n6.逗猫棒可以使撸猫增加心情概率提高30%,\n每撸一次消耗一次逗猫棒" +
			"\n7.修行经验值达到1000以上可进行突破成猫娘,\n经验值越高越容易突破,但迟迟不突破有概率会爆炸" +
			"\n8.品种为猫娘的猫猫可以使用“上传猫猫照片”更换图片",
		PrivateDataFolder: "cybercat",
	}).ApplySingle(ctxext.DefaultSingle)
	getdb = fcext.DoOnceOnSuccess(func(ctx *zero.Ctx) bool {
		err := catdata.db.Open(time.Hour * 24)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return false
		}
		return true
	})
)

func init() {
	engine.OnRegex(`^吸(.*猫)$`).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		typeOfcat := ctx.State["regex_matched"].([]string)[1]
		if typeOfcat == "猫" {
			typeName, temperament, description, url, err := suijineko()
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]: ", err))
				return
			}
			ctx.SendChain(message.Image(url), message.Text("品种: ", typeName,
				"\n气质:\n", temperament, "\n描述:\n", description))
			return
		}
		breeds, ok := typeZH2Breeds[typeOfcat]
		if !ok {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("没有相关该品种的猫图"))
			return
		}
		picurl, err := getPicByBreed(breeds)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]: ", err))
			return
		}
		ctx.SendChain(message.Text("品种: ", typeOfcat), message.Image(picurl))
	})
}

func suijineko() (typeName, temperament, description, url string, err error) {
	data, err := web.GetData(apiURL + "search?has_breeds=1")
	if err != nil {
		return
	}
	picID := gjson.ParseBytes(data).Get("0.id").String()
	picdata, err := web.GetData(apiURL + picID)
	if err != nil {
		return
	}
	name := gjson.ParseBytes(picdata).Get("breeds.0.name").String()
	return typeEN2ZH[name], gjson.ParseBytes(picdata).Get("breeds.0.temperament").String(), gjson.ParseBytes(picdata).Get("breeds.0.description").String(), gjson.ParseBytes(picdata).Get("url").String(), nil
}

func getPicByBreed(catBreed string) (url string, err error) {
	data, err := web.GetData(apiURL + "search?breed_ids=" + catBreed)
	if err != nil {
		return
	}
	return gjson.ParseBytes(data).Get("0.url").String(), nil
}

func (sql *catdb) updateCatInfo(gid string, dbInfo *catInfo) error {
	sql.Lock()
	defer sql.Unlock()
	err := sql.db.Create(gid, &catInfo{})
	if err != nil {
		return err
	}
	return sql.db.Insert(gid, dbInfo)
}

func (sql *catdb) findCatInfo(gid, uid string) (dbInfo *catInfo, err error) {
	sql.Lock()
	defer sql.Unlock()
	err = sql.db.Create(gid, &catInfo{})
	if err != nil {
		return
	}
	if !sql.db.CanFind(gid, "where user = "+uid) {
		return &catInfo{}, nil // 规避没有该用户数据的报错
	}
	var cat catInfo
	dbInfo = &cat
	err = sql.db.Find(gid, dbInfo, "where user = "+uid)
	return
}

// 获取最新状态的猫猫数据
func getNewCatData(gid, uid string) (cat *catInfo, err error) {
	cat, err = catdata.findCatInfo(gid, uid)
	if err != nil {
		return
	}
	if cat == (&catInfo{}) || cat.Name == "" {
		return cat, nil
	}

	now := time.Now()
	elapsed := now.Sub(time.Unix(cat.LastTime, 0)).Seconds()
	cat.SubTime += elapsed

	// 更新心情（随时间缓慢下降）
	moodDecay := elapsed / 3600.0 // 每小时下降1点心情
	cat.Mood = int(math.Max(0, float64(cat.Mood)-moodDecay))

	// 更新饱食度（随时间下降）
	fullnessDecay := elapsed / 1800.0 // 每半小时下降1点饱食度
	cat.Satiety = float64(cat.Satiety) - fullnessDecay

	// 如果饱食度太低，心情下降更快
	if cat.Satiety < 20 {
		cat.Mood = int(math.Max(0, float64(cat.Mood)-moodDecay*2))
	}

	if cat.Satiety < 0 {
		cat.Weight += cat.Satiety * 0.01 // 体重下降
		cat.Satiety = 0
	}

	// 修为值随时间缓慢增长
	cultivationGain := elapsed / 7200.0 // 每2小时增加1点修为
	cat.Experience += int(cultivationGain)

	// 如果心情很好，体重轻微增长
	if cat.Mood > 80 && cat.Satiety > 60 {
		weightGain := elapsed / 86400.0 * 0.01 // 每天约增加0.01kg
		cat.Weight += weightGain
	}

	cat.LastTime = now.Unix()

	catdata.updateCatInfo(gid, cat)
	return
}

func (sql *catdb) catDie(gid, uid string) error {
	sql.Lock()
	defer sql.Unlock()
	return sql.db.Del(gid, "where user = "+uid)
}

func (sql *catdb) getGroupCatdata(gid string) (list []catInfo, err error) {
	sql.RLock()
	defer sql.RUnlock()
	var nekoList []catInfo
	var catList []catInfo
	info := catInfo{}
	err = sql.db.FindFor(gid, &info, "order by Breed DESC, Experience DESC", func() error {
		if info.Name != "" {
			if info.Type == "猫娘" {
				nekoList = append(nekoList, info)
			} else {
				catList = append(catList, info)
			}
		}
		return nil
	})

	list = append(nekoList, catList...)
	return
}

func (sql *catdb) getHomeInfo(uid string) (dbInfo warehouse, err error) {
	sql.Lock()
	defer sql.Unlock()
	err = sql.db.Create("warehouse", &warehouse{})
	if err != nil {
		return
	}
	if !sql.db.CanFind("warehouse", "where user = "+uid) {
		return warehouse{}, nil // 规避没有该用户数据的报错
	}
	err = sql.db.Find("warehouse", &dbInfo, "where user = "+uid)
	return
}

func (sql *catdb) updateHomeInfo(dbInfo *warehouse) error {
	sql.Lock()
	defer sql.Unlock()
	err := sql.db.Create("warehouse", &warehouse{})
	if err != nil {
		return err
	}
	return sql.db.Insert("warehouse", dbInfo)
}

func (c *catInfo) avatar(gid int64) (picbytes []byte, err error) {
	cache := filepath.Join(engine.DataFolder(), "cache")
	if file.IsNotExist(cache) {
		err = os.MkdirAll(cache, 0755)
		if err != nil {
			return web.GetData(c.Picurl)
		}
	}

	imgfloder := filepath.Join(cache, strconv.FormatInt(gid, 10))
	if file.IsNotExist(imgfloder) {
		err = os.MkdirAll(imgfloder, 0755)
		if err != nil {
			return web.GetData(c.Picurl)
		}
	}

	aimgfile := filepath.Join(imgfloder, strconv.FormatInt(c.User, 10)+".gif")
	if file.IsNotExist(aimgfile) {
		err = file.DownloadTo(c.Picurl, aimgfile)
		if err != nil {
			return web.GetData(c.Picurl)
		}
	}
	f, err := os.Open(filepath.Join(file.BOTPATH, aimgfile))
	if err != nil {
		return web.GetData(c.Picurl)
	}
	defer f.Close()
	return io.ReadAll(f)
}
