package steam

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	fcext "github.com/FloatTech/floatbox/ctxext"
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/web"
	ctrl "github.com/FloatTech/zbpctrl"
	trshttp "github.com/fumiama/terasu/http"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

// ----------------------- 远程调用 ----------------------
const (
	gameListURL = "https://partner.steam-api.com/IStoreService/GetAppList/v1/?key=%+v" // 获取所有游戏列表
	gameInfoURL = "https://store.steampowered.com/api/appdetails?appids=%+v"           // 游戏详情页
	ua          = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36 Edg/132.0.0.0"
)

// AppList 游戏列表接口返回的数据结构
type AppList struct {
	Applist struct {
		Apps []App `json:"apps"`
	} `json:"applist"`
}

// App 游戏列表中的每个游戏数据结构
type App struct {
	Appid  int    `json:"appid"`
	Name   string `json:"name"`
	CnName string `json:"cn_name,omitempty"`
}

// AppInfo 游戏详情接口返回的数据结构
type AppInfo struct {
	Success bool `json:"success"`
	Data    struct {
		Type                string `json:"type"`
		Name                string `json:"name"`
		SteamAppid          int    `json:"steam_appid"`
		RequiredAge         int    `json:"required_age"`
		IsFree              bool   `json:"is_free"`
		Dlc                 []int  `json:"dlc"`
		DetailedDescription string `json:"detailed_description"`
		AboutTheGame        string `json:"about_the_game"`
		ShortDescription    string `json:"short_description"`
		SupportedLanguages  string `json:"supported_languages"`
		Reviews             string `json:"reviews"`
		HeaderImage         string `json:"header_image"`
		CapsuleImage        string `json:"capsule_image"`
		CapsuleImagev5      string `json:"capsule_imagev5"`
		Website             string `json:"website"`
		// PcRequirements      struct {
		// 	Minimum string `json:"minimum"`
		// } `json:"pc_requirements"`
		// MacRequirements struct {
		// 	Minimum string `json:"minimum"`
		// } `json:"mac_requirements"`
		// LinuxRequirements struct {
		// 	Minimum string `json:"minimum"`
		// } `json:"linux_requirements"`
		Developers    []string `json:"developers"`
		Publishers    []string `json:"publishers"`
		PriceOverview struct {
			Currency         string `json:"currency"`
			Initial          int    `json:"initial"`
			Final            int    `json:"final"`
			DiscountPercent  int    `json:"discount_percent"`
			InitialFormatted string `json:"initial_formatted"`
			FinalFormatted   string `json:"final_formatted"`
		} `json:"price_overview"`
		Packages      []int `json:"packages"`
		PackageGroups []struct {
			Name                    string `json:"name"`
			Title                   string `json:"title"`
			Description             string `json:"description"`
			SelectionText           string `json:"selection_text"`
			SaveText                string `json:"save_text"`
			DisplayType             int    `json:"display_type"`
			IsRecurringSubscription string `json:"is_recurring_subscription"`
			Subs                    []struct {
				Packageid                int    `json:"packageid"`
				PercentSavingsText       string `json:"percent_savings_text"`
				PercentSavings           int    `json:"percent_savings"`
				OptionText               string `json:"option_text"`
				OptionDescription        string `json:"option_description"`
				CanGetFreeLicense        string `json:"can_get_free_license"`
				IsFreeLicense            bool   `json:"is_free_license"`
				PriceInCentsWithDiscount int    `json:"price_in_cents_with_discount"`
			} `json:"subs"`
		} `json:"package_groups"`
		Platforms struct {
			Windows bool `json:"windows"`
			Mac     bool `json:"mac"`
			Linux   bool `json:"linux"`
		} `json:"platforms"`
		Metacritic struct {
			Score int    `json:"score"`
			URL   string `json:"url"`
		} `json:"metacritic"`
		Categories []struct {
			ID          int    `json:"id"`
			Description string `json:"description"`
		} `json:"categories"`
		Genres []struct {
			ID          string `json:"id"`
			Description string `json:"description"`
		} `json:"genres"`
		Screenshots []struct {
			ID            int    `json:"id"`
			PathThumbnail string `json:"path_thumbnail"`
			PathFull      string `json:"path_full"`
		} `json:"screenshots"`
		Movies []struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Thumbnail string `json:"thumbnail"`
			Webm      struct {
				Num480 string `json:"480"`
				Max    string `json:"max"`
			} `json:"webm"`
			Mp4 struct {
				Num480 string `json:"480"`
				Max    string `json:"max"`
			} `json:"mp4"`
			Highlight bool `json:"highlight"`
		} `json:"movies"`
		Recommendations struct {
			Total int `json:"total"`
		} `json:"recommendations"`
		ReleaseDate struct {
			ComingSoon bool   `json:"coming_soon"`
			Date       string `json:"date"`
		} `json:"release_date"`
		SupportInfo struct {
			URL   string `json:"url"`
			Email string `json:"email"`
		} `json:"support_info"`
		Background         string `json:"background"`
		BackgroundRaw      string `json:"background_raw"`
		ContentDescriptors struct {
			IDs   []int  `json:"ids"`
			Notes string `json:"notes"`
		} `json:"content_descriptors"`
		Ratings struct {
			Kgrb struct {
				Rating      string `json:"rating"`
				Descriptors string `json:"descriptors"`
			} `json:"kgrb"`
			Usk struct {
				Rating string `json:"rating"`
			} `json:"usk"`
			Agcom struct {
				Rating      string `json:"rating"`
				Descriptors string `json:"descriptors"`
			} `json:"agcom"`
			Cadpa struct {
				Rating string `json:"rating"`
			} `json:"cadpa"`
			Dejus struct {
				RatingGenerated string `json:"rating_generated"`
				Rating          string `json:"rating"`
				RequiredAge     string `json:"required_age"`
				Banned          string `json:"banned"`
				UseAgeGate      string `json:"use_age_gate"`
				Descriptors     string `json:"descriptors"`
			} `json:"dejus"`
			SteamGermany struct {
				RatingGenerated string `json:"rating_generated"`
				Rating          string `json:"rating"`
				RequiredAge     string `json:"required_age"`
				Banned          string `json:"banned"`
				UseAgeGate      string `json:"use_age_gate"`
				Descriptors     string `json:"descriptors"`
			} `json:"steam_germany"`
		} `json:"ratings"`
	} `json:"data"`
}

// GameInfo 最终返回给用户的游戏信息数据结构
type GameInfo struct {
	Name                  string
	Appid                 int
	Type                  string
	Date                  string
	ImageSrc              string
	Description           string
	IsFree                bool
	DiscountPct           string
	DiscountOriginalPrice string
	DiscountFinalPrice    string
	Languages             string
	URL                   string
}

var (
	cache = struct {
		sync.RWMutex
		m map[int]string
	}{m: make(map[int]string)}
	gameList    []App
	cfgFile     = engine.DataFolder() + "applist.json"
	getGameList = fcext.DoOnceOnSuccess(func(ctx *zero.Ctx) bool {
		if file.IsExist(cfgFile) {
			reader, err := os.Open(cfgFile)
			if err == nil {
				err = json.NewDecoder(reader).Decode(&gameList)
			}
			if err != nil {
				ctx.SendChain(message.Text("[steam] ERROR: ", err))
				return false
			}
			err = reader.Close()
			if err != nil {
				ctx.SendChain(message.Text("[steam] ERROR: ", err))
				return false
			}
			logrus.Infoln("[steam]获取Steam游戏列表,共", len(gameList), "个游戏")
		} else {
			// 校验密钥是否初始化
			m := ctx.State["manager"].(*ctrl.Control[*zero.Ctx])
			apiKeyMu.Lock()
			defer apiKeyMu.Unlock()
			_ = m.GetExtra(&apiKey)
			if apiKey == "" {
				ctx.SendChain(message.Text("ERROR: 未设置steam apikey"))
				return false
			}
			err := fetchGameList()
			if err != nil {
				ctx.SendChain(message.Text("[steam] ERROR: ", err))
				return false
			}
		}
		return true
	})
)

func init() {
	engine.OnRegex(`^steam (.+)$`, getGameList).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		gameName := ctx.State["regex_matched"].([]string)[1]
		apps := ConcurrentFuzzySearch(gameName)
		if len(apps) == 0 {
			apps = searchWithUpdateList(gameName)
			if len(apps) == 0 {
				ctx.SendChain(message.Text("未找到相关游戏"))
				return
			}
		}
		// if len(apps) > 10 {
		// 	ctx.SendChain(message.Text("找到太多相关游戏，请输入更精确的名称"))
		// 	return
		// }
		// gameInfoList, err := getGameListData(apps)
		// if err != nil {
		// 	ctx.SendChain(message.Text(err))
		// 	return
		// }
		// gameInfo := gameInfoList[0]
		gameInfo, err := getGameWebData(apps[0])
		if err != nil {
			ctx.SendChain(message.Text(err))
			return
		}
		var msg message.Segment
		if gameInfo.Type == "game" {
			msg = message.Text(
				"游戏名: ", gameInfo.Name, "\n",
				"游戏ID: ", gameInfo.Appid, "\n",
				"上线时间: ", gameInfo.Date, "\n",
				"描述: ", gameInfo.Description, "\n",
				"折扣: ", gameInfo.DiscountPct, "\n",
				"原价: ", gameInfo.DiscountOriginalPrice, "\n",
				"现价: ", gameInfo.DiscountFinalPrice, "\n",
				"链接: ", gameInfo.URL,
			)
		} else {
			msg = message.Text(
				"应用名: ", gameInfo.Name, "\n",
				"应用ID: ", gameInfo.Appid, "\n",
				"上线时间: ", gameInfo.Date, "\n",
				"描述: ", gameInfo.Description, "\n",
				"折扣: ", gameInfo.DiscountPct, "\n",
				"原价: ", gameInfo.DiscountOriginalPrice, "\n",
				"现价: ", gameInfo.DiscountFinalPrice, "\n",
				"链接: ", gameInfo.URL,
			)
		}
		if gameInfo.ImageSrc == "" {
			ctx.SendChain(msg)
			return
		}
		ctx.SendChain(message.Image(gameInfo.ImageSrc), msg)
	})
}

func fetchGameList() error {
	var response *http.Response
	url := fmt.Sprintf(gameListURL, apiKey)
	print(url)
	response, err := trshttp.Get(url)
	if err != nil {
		response, err = http.Get(url)
	}
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		s := fmt.Sprintf("status code: %d", response.StatusCode)
		err = errors.New(s)
		return err
	}
	defer response.Body.Close()

	var appList AppList
	err = json.NewDecoder(response.Body).Decode(&appList)
	if err != nil {
		return err
	}

	logrus.Infoln("[steam]共读取到", len(appList.Applist.Apps), "个应用")

	// 并发获取中文名称
	apps := appList.Applist.Apps
	gameList = make([]App, 0, len(apps))

	// 使用worker pool处理
	var wg sync.WaitGroup
	ch := make(chan App, len(apps))
	resultCh := make(chan App, len(apps))

	// 启动worker
	workerCount := 5
	for range workerCount {
		wg.Add(1)
		go worker(ch, resultCh, &wg)
	}

	// 发送任务
	for _, app := range apps {
		ch <- app
	}
	close(ch)

	// 等待所有worker完成
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 收集结果
	for app := range resultCh {
		gameList = append(gameList, app)
	}

	// 保存结果
	output, err := json.MarshalIndent(gameList, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(cfgFile, output, 0644)
	if err != nil {
		return err
	}

	logrus.Debugln("处理完成，结果已保存到", cfgFile)
	return err
}

// worker 处理获取中文名称的工作
func worker(ch <-chan App, resultCh chan<- App, wg *sync.WaitGroup) {
	defer wg.Done()

	for app := range ch {
		cache.RLock()
		cnName, ok := cache.m[app.Appid]
		cache.RUnlock()

		if ok {
			app.CnName = cnName
			resultCh <- app
			continue
		}

		gameInfo, err := getGameWebData(app.Appid)
		if err != nil {
			logrus.Warnln("获取游戏信息失败: ", err)
			continue
		}
		cnName = gameInfo.Name

		cache.Lock()
		cache.m[app.Appid] = cnName
		cache.Unlock()

		app.CnName = cnName
		resultCh <- app

		time.Sleep(100 * time.Millisecond)
	}
}

func searchWithUpdateList(keyword string) (apps []int) {
	err := fetchGameList()
	if err != nil {
		logrus.Warn("更新Steam游戏列表失败: ", err)
		return
	}
	apps = ConcurrentFuzzySearch(keyword)
	return
}

func ConcurrentFuzzySearch(keyword string) []int {
	keywordLower := strings.ToLower(keyword)
	var results []App
	var finalResults []int

	for _, app := range gameList {
		// 在英文名称中搜索
		if strings.Contains(strings.ToLower(app.Name), keywordLower) {
			results = append(results, app)
			continue
		}
		// 在中文名称中搜索
		if strings.Contains(strings.ToLower(app.CnName), keywordLower) {
			results = append(results, app)
		}
	}

	for _, app := range results {
		finalResults = append(finalResults, app.Appid)
	}
	// 优先返回老游戏
	slices.Sort(finalResults)
	return finalResults
}

func getGameWebData(appid int) (data GameInfo, err error) {
	appidStr := strconv.Itoa(appid)
	url := fmt.Sprintf(gameInfoURL, appidStr)
	apiResponse, err := web.RequestDataWithHeaders(web.NewDefaultClient(), url, "GET", func(r *http.Request) error {
		r.Header.Add("Cookie", "steamCountry=CN;")
		r.Header.Set("accept-language", "zh-CN,zh;q=0.9")
		r.Header.Set("User-Agent", ua)
		return nil
	}, nil)
	if err != nil {
		return
	}
	var apiData map[string]AppInfo
	logrus.Warnln("___________\n", appid, "\n", string(apiResponse), "\n___________")
	err = json.Unmarshal(apiResponse, &apiData)
	if err != nil {
		return
	}

	data = GameInfo{
		Name:        apiData[appidStr].Data.Name,
		Appid:       apiData[appidStr].Data.SteamAppid,
		Type:        apiData[appidStr].Data.Type,
		Date:        apiData[appidStr].Data.ReleaseDate.Date,
		Description: apiData[appidStr].Data.ShortDescription,
		Languages:   apiData[appidStr].Data.SupportedLanguages,
		IsFree:      apiData[appidStr].Data.IsFree,
		URL:         fmt.Sprintf("https://store.steampowered.com/app/%d", apiData[appidStr].Data.SteamAppid),
	}
	if data.IsFree {
		data.DiscountPct = "/"
		data.DiscountOriginalPrice = "免费"
		data.DiscountFinalPrice = "-"
	} else {
		data.DiscountPct = fmt.Sprintf("-%d%%", apiData[appidStr].Data.PriceOverview.DiscountPercent)
		data.DiscountOriginalPrice = apiData[appidStr].Data.PriceOverview.InitialFormatted
		data.DiscountFinalPrice = apiData[appidStr].Data.PriceOverview.FinalFormatted
	}
	if apiData[appidStr].Data.HeaderImage != "" {
		data.ImageSrc = apiData[appidStr].Data.HeaderImage
	}
	return
}
