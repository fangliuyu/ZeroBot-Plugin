package steam

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/web"
	trshttp "github.com/fumiama/terasu/http"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

// ----------------------- 远程调用 ----------------------
const (
	gameListUrl = "https://api.steampowered.com/ISteamApps/GetAppList/v2/"   // 获取所有游戏列表
	gameInfoUrl = "https://store.steampowered.com/api/appdetails?appids=%+v" // 游戏详情页
	ua          = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36 Edg/132.0.0.0"
)

type GameList struct {
	Applist struct {
		Apps []App `json:"apps"`
	} `json:"applist"`
}

type App struct {
	Appid int    `json:"appid"`
	Name  string `json:"name"`
}

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
			Ids   []int  `json:"ids"`
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
	cfgFile  = engine.DataFolder() + "applist.json"
	gamelist GameList
)

func init() {
	if file.IsExist(cfgFile) {
		reader, err := os.Open(cfgFile)
		if err == nil {
			err = json.NewDecoder(reader).Decode(&gamelist)
		}
		if err != nil {
			panic(err)
		}
		err = reader.Close()
		if err != nil {
			panic(err)
		}
	} else {
		err := fetchGameList()
		if err != nil {
			logrus.Warn("获取Steam游戏列表失败: ", err)
		}
	}
	engine.OnRegex(`steam (.+)$`).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		gameName := ctx.State["regex_matched"].([]string)[1]
		logrus.Warn("___________\n\n", gameName, "\n\n___________")
		apps := ConcurrentFuzzySearch(gamelist, gameName)
		if len(apps) == 0 {
			apps = updateList(gameName)
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
	response, err := trshttp.Get(gameListUrl)
	if err != nil {
		response, err = http.Get(gameListUrl)
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

	err = json.NewDecoder(response.Body).Decode(&gamelist)
	if err != nil {
		return err
	}

	reader, err := os.Create(cfgFile)
	if err == nil {
		err = json.NewEncoder(reader).Encode(&gamelist)
	}
	return err
}

func updateList(keyword string) (apps []int) {
	err := fetchGameList()
	if err != nil {
		logrus.Warn("更新Steam游戏列表失败: ", err)
		return
	}
	apps = ConcurrentFuzzySearch(gamelist, keyword)
	return
}

func ConcurrentFuzzySearch(data GameList, keyword string) []int {
	var wg sync.WaitGroup
	results := make(chan int, len(data.Applist.Apps))
	var mutex sync.Mutex
	var finalResults []int

	for _, app := range data.Applist.Apps {
		wg.Add(1)
		go func(a App) {
			defer wg.Done()
			if strings.Contains(strings.ToLower(a.Name), strings.ToLower(keyword)) {
				mutex.Lock()
				results <- a.Appid
				mutex.Unlock()
			}
		}(app)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for app := range results {
		finalResults = append(finalResults, app)
	}
	// 优先返回老游戏
	sort.Slice(finalResults, func(i, j int) bool {
		return finalResults[i] < finalResults[j]
	})
	return finalResults
}

// 并发获取多个游戏信息(预留)
//
//lint:ignore U1000 Ignore unused function temporarily for debugging
func getGameListData(data []App) (gameInfoList []GameInfo, err error) {
	var wg sync.WaitGroup
	results := make(chan GameInfo, len(data))
	var mutex sync.Mutex

	for _, app := range data {
		wg.Add(1)
		go func(a App) {
			defer wg.Done()
			gameinfo, webErr := getGameWebData(a.Appid)
			mutex.Lock()
			if webErr != nil {
				if err != nil {
					err = errors.New(
						err.Error() + ";\n" +
							"获取《" + a.Name + "》游戏信息失败: " + webErr.Error(),
					)
				} else {
					err = webErr
				}
				return
			}
			results <- gameinfo
			mutex.Unlock()
		}(app)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for gameinfo := range results {
		gameInfoList = append(gameInfoList, gameinfo)
	}
	return
}

func getGameWebData(appid int) (data GameInfo, err error) {
	appidStr := strconv.Itoa(appid)
	url := fmt.Sprintf(gameInfoUrl, appidStr)
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
