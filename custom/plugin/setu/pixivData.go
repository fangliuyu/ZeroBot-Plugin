// Package setutime 来份涩图
package setutime

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync"

	fileutil "github.com/FloatTech/floatbox/file"
	zbpmath "github.com/FloatTech/floatbox/math"
	"github.com/FloatTech/floatbox/process"
	sql "github.com/FloatTech/sqlite"
	"github.com/sirupsen/logrus"
)

var DefaultList = []string{"游戏王", "东方", "绯弹的亚里亚", "二次元"}

// ImgPool 图片缓冲池实例
var ImgPool = &imgpool{
	path: CacheDir,
	max:  10,
	pool: make(map[string][]string),
}

// imgpool 图片缓冲池
type imgpool struct {
	// 数据库相关
	db   sql.Sqlite
	dbmu sync.RWMutex
	// 图片池相关
	path   string
	max    int
	pool   map[string][]string
	poolmu sync.Mutex
}

// List 返回数据库中所有的table名称
func (p *imgpool) List() (l []string) {
	var err error
	p.dbmu.RLock()
	defer p.dbmu.RUnlock()
	l, err = p.db.ListTables()
	if err != nil || len(l) == 0 {
		l = DefaultList
	} else {
		copy(l, DefaultList)
	}
	return l
}

// Size 返回缓冲池指定类型的现有大小
func (p *imgpool) Size(imgtype string) int {
	p.poolmu.Lock()
	defer p.poolmu.Unlock()
	return len(p.pool[imgtype])
}

// push 向缓冲池添加图片
func (p *imgpool) push(imgtype string, illust *Illust) error {
	if len(illust.ImageUrls) == 0 {
		return errors.New("nil image url")
	}

	type downloadResult struct {
		index int
		path  string
		err   error
	}

	results := make(chan downloadResult, len(illust.ImageUrls))
	var wg sync.WaitGroup

	// 并发下载所有页面
	for index := range illust.MaxPager {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			path := illust.Path(idx)
			if path == "" {
				results <- downloadResult{index: idx, err: errors.New("无法获取到文件路径")}
				return
			}

			f := fileutil.BOTPATH + "/" + path

			// 检查文件是否已存在
			if fileutil.IsExist(f) {
				logrus.Infoln("PID:", illust.Pid, "的第", idx, "页已存在，跳过下载")
				results <- downloadResult{index: idx, path: f, err: nil}
				return
			}
			logrus.Infoln("正在下载图片 PID:", illust.Pid, "的第", idx, "页")
			// 下载图片
			if err := illust.DownloadToCache(idx); err != nil {
				results <- downloadResult{index: idx, err: err}
				return
			}
			results <- downloadResult{index: idx, path: f, err: nil}
		}(index)
	}

	// 等待所有下载完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	var errs []error
	// p.poolmu.Lock()

	for result := range results {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("page %d: %w", result.index, result.err))
			continue
		}

		// 检查年龄限制
		if _, ok := illust.ImageUrls[result.index]; !ok {
			logrus.Infoln("图片", illust.Pid, "的第", result.index, "页没有图片链接，已跳过添加到"+imgtype+"池子")
			continue
		}
		if len(illust.AgeLimit) != 0 && slices.Contains(illust.AgeLimit, result.index) {
			logrus.Infoln("图片", illust.Pid, "的第", result.index, "页有年龄限制，已跳过添加到"+imgtype+"池子")
			continue
		}

		p.poolmu.Lock()
		p.pool[imgtype] = append(p.pool[imgtype], result.path)
		p.poolmu.Unlock()
		logrus.Infoln("图片", illust.Pid, "的第", result.index, "页已添加到"+imgtype+"池子")
	}
	// p.poolmu.Unlock()

	if len(errs) > 0 {
		return fmt.Errorf("下载失败: %v", errs)
	}
	return nil
}

// Pop 从缓冲池中弹出一张图片的路径
func (p *imgpool) Pop(imgtype string) (path string) {
	p.poolmu.Lock()
	defer p.poolmu.Unlock()

	if len(p.pool[imgtype]) == 0 {
		return
	}
	path = p.pool[imgtype][0]
	p.pool[imgtype] = p.pool[imgtype][1:]
	return
}

// Fill 补充池子
func (p *imgpool) Fill(imgtype string) {
	times := zbpmath.Max(p.max-p.Size(imgtype), 2)
	logrus.Infoln("开始补充", imgtype, "池子", "数量: ", times, "次")
	breaked := false
	for i := range times {
		logrus.Infoln("正在补充第", i+1, "次")
		illust := &Illust{}
		// 查询出一张图片
		p.dbmu.RLock()
		if err := p.db.Pick(imgtype, illust); err != nil {
			if errors.Is(err, sql.ErrNullResult) {
				logrus.Infoln("没有更多数据了，停止补充", imgtype, "池子")
				breaked = true
			} else {
				logrus.Warningln("查询图片失败: ", err)
			}
		}
		p.dbmu.RUnlock()
		if breaked {
			break
		}
		if len(illust.ImageUrls) == 0 {
			continue
		}
		logrus.Infoln("抽到了", illust.Pid)
		// 向缓冲池添加一张图片
		if err := p.push(imgtype, illust); err != nil {
			logrus.Warningln("添加图片失败: ", err)
			continue
		}
		process.SleepAbout1sTo2s()
	}
	if p.Size(imgtype) < 3 && !breaked {
		logrus.Warningln("补充池子失败，池子仍然为空: ", imgtype)
		p.Fill(imgtype)
	} else {
		logrus.Infoln("补充池子完成", imgtype+"池子当前大小: ", p.Size(imgtype))
	}
}

// StringToIntMap 是一个 map[int]string 的别名，用于数据库序列化和反序列化
func (p *imgpool) AddLocal(imgtype string, pid int64, path ...string) (err error) {
	p.dbmu.Lock()
	defer p.dbmu.Unlock()
	if err := p.db.Create(imgtype, &Illust{}); err != nil {
		return err
	}
	illust := &Illust{
		Pid: pid,
	}
	oldIllust := &Illust{Pid: illust.Pid}
	err = p.db.Find(imgtype, oldIllust, "WHERE pid = ?", illust.Pid)

	if err == nil {
		if reflect.DeepEqual(oldIllust, illust) {
			logrus.Infof("图片 PID: %d 已存在且数据相同，跳过添加", illust.Pid)
			return nil
		}
		// 找到已存在的记录，合并数据
		logrus.Infof("图片 PID: %d 已存在，正在合并数据", illust.Pid)
		illust = p.mergeIllust(oldIllust, illust)
	}
	for _, p := range path {
		if fileutil.IsNotExist(p) {
			continue
		}
		len := len(illust.ImageUrls)
		if len == 0 {
			illust.ImageUrls = map[int]string{
				0: p,
			}
		} else {
			illust.ImageUrls[len] = p
		}
		break
	}
	// 添加插画到对应的数据库table
	return p.db.Insert(imgtype, illust)
}

// AddIllust 向数据库和缓冲池中添加图片
func (p *imgpool) AddIllust(imgtype string, illust *Illust) (err error) {
	// 参数验证
	if illust == nil {
		return errors.New("illust is nil")
	}
	if len(illust.ImageUrls) == 0 {
		return errors.New("nil image url")
	}

	p.dbmu.Lock()
	defer p.dbmu.Unlock()

	// 创建表（如果不存在）
	if err := p.db.Create(imgtype, &Illust{}); err != nil {
		return fmt.Errorf("create table failed: %w", err)
	}

	// 查询已存在的记录（只查询一次）
	oldIllust := &Illust{Pid: illust.Pid}
	err = p.db.Find(imgtype, oldIllust, "WHERE pid = ?", illust.Pid)

	if err == nil {
		if reflect.DeepEqual(oldIllust, illust) {
			logrus.Infof("图片 PID: %d 已存在且数据相同，跳过添加", illust.Pid)
			return nil
		}
		// 找到已存在的记录，合并数据
		logrus.Infof("图片 PID: %d 已存在，正在合并数据", illust.Pid)
		illust = p.mergeIllust(oldIllust, illust)
	}

	err = p.db.Insert(imgtype, illust)
	if err == nil {
		logrus.Infof("添加图片 PID: %d 到分类 %s成功", illust.Pid, imgtype)
	}
	return err
}

// mergeIllust 合并两个 Illust，返回新的 Illust
func (p *imgpool) mergeIllust(old, new *Illust) *Illust {
	result := &Illust{
		Pid:        new.Pid,
		Title:      new.Title,
		Caption:    new.Caption,
		UID:        new.UID,
		Author:     new.Author,
		UploadDate: new.UploadDate,
	}

	// 合并 MaxPager
	if new.MaxPager > old.MaxPager {
		result.MaxPager = new.MaxPager
	} else {
		result.MaxPager = old.MaxPager
	}

	// 合并 Tags（去重）
	tagMap := make(map[string]bool)
	for _, tag := range old.Tags {
		tagMap[tag] = true
	}
	for _, tag := range new.Tags {
		tagMap[tag] = true
	}
	result.Tags = make([]string, 0, len(tagMap))
	for tag := range tagMap {
		result.Tags = append(result.Tags, tag)
	}

	// 合并 ImageUrls
	result.ImageUrls = make(map[int]string)
	for k, v := range old.ImageUrls {
		result.ImageUrls[k] = v
	}
	for k, v := range new.ImageUrls {
		result.ImageUrls[k] = v // 新数据覆盖旧数据
	}

	// 合并 AgeLimit（去重）
	ageMap := make(map[int]bool)
	for _, age := range old.AgeLimit {
		ageMap[age] = true
	}
	for _, age := range new.AgeLimit {
		ageMap[age] = true
	}
	result.AgeLimit = make([]int, 0, len(ageMap))
	for age := range ageMap {
		result.AgeLimit = append(result.AgeLimit, age)
	}

	return result
}

// Remove 从数据库和缓冲池中删除图片
func (p *imgpool) Remove(imgtype string, id int64) error {
	p.dbmu.Lock()
	defer p.dbmu.Unlock()
	return p.db.Del(imgtype, "WHERE pid = ?", id)
}

// GetIllustInfo 获取插画信息
func (p *imgpool) GetIllustInfo(imgtype string, id int64) (illust *Illust, err error) {
	p.dbmu.RLock()
	defer p.dbmu.RUnlock()
	illust = &Illust{}
	err = p.db.Find(imgtype, illust, "WHERE pid = ?", id)
	return
}

// AddAPItoPool 从API获取图片并添加到池子
func AddAPItoPool(imgtype string, tags ...string) {
	if ImgPool.Size(imgtype) < 5 {
		ImgPool.Fill(imgtype)
	}
	var tagList strings.Builder
	if len(tags) != 0 {
		for _, tag := range tags {
			tagList.WriteString("&tag=" + url.QueryEscape(tag))
		}
	} else {
		tagList.WriteString("&tag=" + url.QueryEscape(imgtype))
	}
	urlAPI := fmt.Sprintf(SreachURL, tagList.String())
	data, err := GetAPIData(urlAPI)
	if err != nil {
		logrus.Warningln("获取数据失败: ", err)
		return
	}

	if len(data) == 0 {
		logrus.Warningln("没有获取到任何数据，可能标签不存在或网络问题")
		return
	}

	illusts, err := TransListToIllust(data)
	if err != nil {
		logrus.Warningln("数据转换失败: ", err)
		// 即使部分失败，也尝试添加成功的
		if len(illusts) == 0 {
			return
		}
	}
	logrus.Infof("API 获取到%s数据%d个", imgtype, len(illusts))

	for _, illust := range illusts {
		illust.Caption = imgtype
		err = ImgPool.AddIllust(imgtype, illust)
		if err != nil {
			logrus.Warningf("添加图片 PID: %d 到分类 %s失败: %v", illust.Pid, imgtype, err)
			continue
		}
	}
}
