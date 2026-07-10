// Package setutime 来份涩图
package setutime

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/FloatTech/floatbox/binary"
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/math"
	"github.com/FloatTech/floatbox/process"
	"github.com/FloatTech/floatbox/web"
	trshttp "github.com/fumiama/terasu/http"
	"github.com/sirupsen/logrus"
)

// 配置参数
const (
	// CacheDir ...
	CacheDir       = "data/pixiv/"
	slicecap int64 = 65536
)

func init() {
	err := os.MkdirAll(CacheDir, 0755)
	if err != nil {
		panic(err)
	}
}

// Illust 插画结构体
type Illust struct {
	Pid        int64          `db:"Pid"`
	Title      string         `db:"Title"`
	Caption    string         `db:"caption"`
	UID        int64          `db:"UID"`
	Author     string         `db:"Author"`
	Tags       StringSlice    `db:"Tags"`
	MaxPager   int            `db:"MaxPager"`
	ImageUrls  StringToIntMap `db:"ImageUrls"`
	AgeLimit   IntSlice       `db:"AgeLimit"`
	UploadDate int64          `db:"UploadDate"`
}

// 为 []string 实现 Scanner 和 Valuer
type StringSlice []string

func (s *StringSlice) Scan(value any) error {
	if value == nil {
		*s = []string{}
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
	return json.Unmarshal(data, s)
}

func (s StringSlice) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	return json.Marshal(s)
}

// 为 []int 实现 Scanner 和 Valuer
type IntSlice []int

func (i *IntSlice) Scan(value any) error {
	if value == nil {
		*i = []int{}
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
	return json.Unmarshal(data, i)
}

func (i IntSlice) Value() (driver.Value, error) {
	if len(i) == 0 {
		return "[]", nil
	}
	return json.Marshal(i)
}

// 为 map[int]string 实现 Scanner 和 Valuer
type StringToIntMap map[int]string

func (m *StringToIntMap) Scan(value any) error {
	if value == nil {
		*m = make(StringToIntMap)
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}

	// JSON 的 key 只能是字符串，需要转换
	var temp map[string]string
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	*m = make(StringToIntMap)
	for k, v := range temp {
		var key int
		fmt.Sscanf(k, "%d", &key)
		(*m)[key] = v
	}
	return nil
}

func (m StringToIntMap) Value() (driver.Value, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	// 转换为 map[string]string 以便 JSON 序列化
	temp := make(map[string]string)
	for k, v := range m {
		temp[fmt.Sprintf("%d", k)] = v
	}
	return json.Marshal(temp)
}

// Path 图片本地缓存路径
func (i *Illust) Path(page int) string {
	u := ""
	if url, exists := i.ImageUrls[page]; exists {
		u = url
	} else if len(i.ImageUrls) > 0 {
		var firstUrl string
		for _, url := range i.ImageUrls {
			firstUrl = url
			break
		}
		if firstUrl != "" {
			u = strings.ReplaceAll(firstUrl, "_p0.", "_p"+strconv.Itoa(page)+".")
			logrus.Warningln("没有找到第", page, "页的URL，使用第0页的URL替代: ", u)
		}
	}
	if u == "" {
		return ""
	}
	_, fileName := filepath.Split(u)
	masterDir := filepath.Join(CacheDir, i.Caption)
	err := os.MkdirAll(masterDir, 0755)
	if err != nil {
		logrus.Errorln("创建目录失败: ", err)
		return ""
	}
	f := filepath.Join(masterDir, fileName)
	logrus.Infoln("构建图片路径: ", f)
	return f
}

// DownloadToCache 多线程下载第 page 页到 i.Path(page), 返回 error
func (i *Illust) DownloadToCache(page int) error {
	url := ""
	picFile := ""
	if u, exists := i.ImageUrls[page]; exists {
		url = u
		picFile = i.Path(page)
	} else if len(i.ImageUrls) > 0 {
		url = strings.ReplaceAll(i.ImageUrls[0], "_p0.", "_p"+strconv.Itoa(page)+".")
		_, fileName := filepath.Split(url)
		masterDir := filepath.Join(CacheDir, i.Caption)
		err := os.MkdirAll(masterDir, 0755)
		if err != nil {
			logrus.Errorln("创建目录失败: ", err)
			return err
		}
		picFile = filepath.Join(masterDir, fileName)
	}
	if url == "" {
		return errors.New("没有找到图片URL")
	}
	return DownloadPicTo(url, picFile)
}

// DownloadPicTo 下载
func DownloadPicTo(u, path string) (err error) {
	if file.IsExist(path) {
		logrus.Warningln("图片已存在: " + path + ", 跳过下载")
		return nil
	}
	if _, err := os.Stat(filepath.Dir(path)); errors.Is(err, os.ErrNotExist) {
		logrus.Infoln("目录不存在，正在创建: ", filepath.Dir(path))
		err = os.MkdirAll(filepath.Dir(path), 0755)
		if err != nil {
			return err
		}
	}
	// 获取IP地址
	domain, err := url.Parse(u)
	if err != nil {
		return err
	}

	header := http.Header{
		"Host":          []string{domain.Host},
		"User-Agent":    []string{"Mozilla/5.0 (Windows NT 6.1; WOW64; rv:6.0) Gecko/20100101 Firefox/6.0"},
		"Cache-Control": []string{"no-cache"},
	}

	// 请求 Header
	headreq, err := http.NewRequest("HEAD", u, nil)
	if err != nil {
		return err
	}
	headreq.Header = header.Clone()
	client := web.NewPixivClient()
	headresp, err := client.Do(headreq)
	if err != nil {
		return err
	}
	contentLengthStr := headresp.Header.Get("Content-Length")
	var contentlength int64

	if contentLengthStr == "" {
		// 如果没有 Content-Length，使用传统方式单线程下载
		logrus.Warnf("图片 %s 没有 Content-Length，使用单线程下载", u)
		return downloadSingleThread(u, path)
	}

	contentlength, err = strconv.ParseInt(contentLengthStr, 10, 64)
	if err != nil {
		logrus.Warnf("解析 Content-Length 失败: %v，使用单线程下载", err)
		return downloadSingleThread(u, path)
	}
	logrus.Infoln("图片大小: ", contentlength, "字节")
	// 多线程下载
	return downloadMultiThread(u, path, header, contentlength)
}

func downloadSingleThread(url, path string) (err error) {
	var resp *http.Response
	resp, err = trshttp.Get(url)
	if err != nil {
		logrus.Warningln("trshttp:", err)
		resp, err = http.Get(url)
	}
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败, HTTP状态码: %d", resp.StatusCode)
	}

	if resp.Body == http.NoBody {
		return
	}

	// 获取内容长度（如果可用）
	contentLength := resp.ContentLength
	var downloaded int64

	out, err := os.Create(path)
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

	return
}

func downloadMultiThread(u, path string, header http.Header, contentlength int64) error {
	// 多线程下载
	client := web.NewPixivClient()
	var wg sync.WaitGroup
	var start int64
	errs := make(chan error, 8)
	allindex := contentlength/slicecap + 1
	buf := make(net.Buffers, 0, allindex)
	writers := make([]*binary.Writer, 0, allindex)
	index := 0
	finish := 0
	for end := math.Min(start+slicecap, contentlength); ; end += slicecap {
		wg.Add(1)
		buf = append(buf, nil)
		writers = append(writers, nil)
		if end > contentlength {
			end = contentlength
		}
		go func(start int64, end int64, index int) {
			// fmt.Println(contentlength, start, end)
			for range 3 {
				req, err := http.NewRequest("GET", u, nil)
				if err != nil {
					errs <- err
					process.SleepAbout1sTo2s()
					continue
				}
				req.Header = header.Clone()
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end-1))
				resp, err := client.Do(req)
				if err != nil {
					errs <- err
					process.SleepAbout1sTo2s()
					continue
				}
				w := binary.SelectWriter()
				_, err = io.CopyN(w, resp.Body, end-start)
				_ = resp.Body.Close()
				if err != nil {
					errs <- err
					binary.PutWriter(w)
					process.SleepAbout1sTo2s()
					continue
				}
				buf[index] = w.Bytes()
				writers[index] = w
				finish += 1
				logrus.Infoln("图片下载完成: ", int64(finish)*100/allindex, "%")
				break
			}
			wg.Done()
		}(start, end, index)
		if end == contentlength {
			break
		}
		start = end
		index++
	}
	msg := ""
	go func() {
		for err := range errs {
			msg += err.Error() + "&"
		}
	}()
	wg.Wait()
	close(errs)
	var err error
	if msg != "" {
		err = errors.New(msg[:len(msg)-1])
	} else {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, &buf)
		_ = f.Close()
		if err != nil {
			_ = os.Remove(path)
		}
	}
	for _, w := range writers {
		if w != nil {
			binary.PutWriter(w)
		}
	}
	return err
}
