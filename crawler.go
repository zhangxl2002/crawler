package main

import (
	"fmt"
	"time"
)

type Fetcher interface {
	// Fetch 返回 URL 的 body 内容，并且将在这个页面上找到的 URL 放到一个 slice 中。
	Fetch(url string) (body string, urls []string, err error)
}
type UrlPool struct {
	ch   chan bool
	m    map[string]bool
	pool chan Task
}
type Worker struct {
	urlPool *UrlPool
	fetcher Fetcher
}
type Task struct {
	url   string
	depth int64
}

func NewUrlPool(originUrl string, depth int64) *UrlPool {
	urlPool := &UrlPool{
		// 整个数据结构的锁
		ch: make(chan bool, 1),
		// 记录每个url是否已经处理过
		m: make(map[string]bool),
		// 当前待处理的url的集合
		pool:    make(chan Task, 100),
		taskCnt: make(chan int, 100),
	}
	// 初始的待处理url
	urlPool.pool <- Task{
		url:   originUrl,
		depth: depth,
	}
	return urlPool
}

// 用于worker通过url获得新的url后，将它们加入待处理的结合
func (urlPool *UrlPool) Put(tasks []Task) {
	urlPool.ch <- true
	for _, t := range tasks {
		if _, ok := urlPool.m[t.url]; ok {
			continue
		}
		urlPool.pool <- t
	}
	<-urlPool.ch
}

// 用于worker领取新的url
func (urlPool *UrlPool) Get() (t Task) {
	// 一定要先确定有剩余待处理的任务，再获取urlPool的锁。否则可能出现，在没有剩余待处理任务的时候获取了锁，等待生产者产生新的任务。而生产者调用Put同样需要获得锁，进而陷入死锁
	<-urlPool.taskCnt
	urlPool.ch <- true
	t = <-urlPool.pool
	urlPool.m[t.url] = true
	t.depth -= 1
	<-urlPool.ch
	return
}

func (worker *Worker) Work() {
	go func() {
		for true {
			task := worker.urlPool.Get()
			if task.depth > 0 {
				body, urls, err := worker.fetcher.Fetch(task.url)
				// _, urls, err := worker.fetcher.Fetch(task.url)
				if err != nil {
					fmt.Println(err)
					continue
				}
				fmt.Printf("found: %s %q\n", task.url, body)
				// fmt.Printf("found: %s\n", task.url)
				tasks := make([]Task, 0)
				for _, url := range urls {
					tasks = append(tasks, Task{
						url:   url,
						depth: task.depth,
					})
				}
				worker.urlPool.Put(tasks)
			}
		}
	}()
}

// Crawl 使用 fetcher 从某个 URL 开始递归的爬取页面，直到达到最大深度。
func Crawl_origin(url string, depth int, fetcher Fetcher) {
	// TODO: 并行的抓取 URL。
	// TODO: 不重复抓取页面。
	// 下面并没有实现上面两种情况：
	if depth <= 0 {
		return
	}
	body, urls, err := fetcher.Fetch(url)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("found: %s %q\n", url, body)
	for _, u := range urls {
		Crawl(u, depth-1, fetcher)
	}
	return
}
func Crawl(url string, depth int, fetcher Fetcher) {
	urlPool := NewUrlPool(url, int64(depth))
	for i := 1; i < 5; i++ {
		worker := &Worker{
			urlPool: urlPool,
			fetcher: fetcher,
		}
		worker.Work()
	}
	time.Sleep(time.Second * 100)
}

func main() {
	Crawl("https://golang.org/", 4, fetcher)
}

// fakeFetcher 是返回若干结果的 Fetcher。
type fakeFetcher map[string]*fakeResult

type fakeResult struct {
	body string
	urls []string
}

func (f fakeFetcher) Fetch(url string) (string, []string, error) {
	if res, ok := f[url]; ok {
		return res.body, res.urls, nil
	}
	return "", nil, fmt.Errorf("not found: %s", url)
}

// fetcher 是填充后的 fakeFetcher。
var fetcher = fakeFetcher{
	"https://golang.org/": &fakeResult{
		"The Go Programming Language",
		[]string{
			"https://golang.org/pkg/",
			"https://golang.org/cmd/",
		},
	},
	"https://golang.org/pkg/": &fakeResult{
		"Packages",
		[]string{
			"https://golang.org/",
			"https://golang.org/cmd/",
			"https://golang.org/pkg/fmt/",
			"https://golang.org/pkg/os/",
		},
	},
	"https://golang.org/pkg/fmt/": &fakeResult{
		"Package fmt",
		[]string{
			"https://golang.org/",
			"https://golang.org/pkg/",
		},
	},
	"https://golang.org/pkg/os/": &fakeResult{
		"Package os",
		[]string{
			"https://golang.org/",
			"https://golang.org/pkg/",
		},
	},
}
