//测试文件一号：用来生成prometheus上的自定义数据。使用方法是用go build编译这个文件生成二进制文件，
//然后上传到https://github.com/wtysos11/PrometheusMonitor上替换掉可执行文件test，此时docker hub上会自动创建镜像
//用该镜像创建deployment，暴露8080端口，注意在template-annotation处加上prometheus.io/scrape: "true"与prometheus.io/port= "8080"
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"fmt"
)

var (
	addr              = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
	appName 		  = flag.String("appName", "productPage", "The name of app.")
)

var (
	// Create a summary to track fictional interservice RPC latencies for three
	// distinct services with different latency distributions. These services are
	// differentiated via a "service" label.
	httpDurationSeconds = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "my_http_durations_seconds",
			Help:       "Http duration for spefic applications per seconds",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"service"},
	)
	httpDurationMinutes = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "my_http_durations_minutes",
			Help:       "Http duration for spefic applications per minutes",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"service"},
	)
	// The same as above, but now as a histogram, and only for the normal
	// distribution. The buckets are targeted to the parameters of the
	// normal distribution, with 20 buckets centered on the mean, each
	// half-sigma wide.
	httpDurationsSecondHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "my_http_durations_histogram_seconds",
		Help:    "specific applications latency distributions.",
		Buckets: prometheus.LinearBuckets(0, 1, 2000),
	})
	httpDurationsMinuteHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "my_http_durations_histogram_minute",
		Help:    "specific applications latency distributions. Per minute",
		Buckets: prometheus.LinearBuckets(0, 1, 2000),
	})
	GaugeRes200 = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "response_minute_200_ratio",
        Help: "ratio of response with status code 200 in a minute",
	})
	GaugeRes5xx = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "response_minute_5xx_ratio",
        Help: "ratio of response with status code 5xx in a minute",
    })
)

//用于实现平均响应时间
type Recorder struct{
	Record	[60]float64
	Counter	int	//一分钟内的时间计数
	Res5xxCounter int	//返回值为5xx的数量
	Res200Counter int	//返回值为200的数量
}

//全局级别的时钟，用来记录
var recorder Recorder

//err!=nil时,Error = True
func flashRecord(start,end int64,Error bool){
	//记录秒级数据
	responsetime := (float64(end)-float64(start))/1000000
	httpDurationSeconds.WithLabelValues(*appName).Observe(responsetime)
	httpDurationsSecondHistogram.Observe(responsetime)

	//如果有错误，则平均时间不计入
	if(Error){
		recorder.Record[recorder.Counter] = -1
		recorder.Counter += 1
	} else{
		recorder.Record[recorder.Counter] = responsetime
		recorder.Counter += 1
	}

	fmt.Println(start/ int64(time.Millisecond),recorder.Counter,responsetime)//进入时间ms,计数器，响应时间

	//计数器，60一次求平均响应时间
	//对于5xx和200数据进行清零
	if recorder.Counter>=60{
		var avgresponseTime float64
		avgresponseTime = 0
		totalNum := 0
		for i := 0;i<recorder.Counter;i++{
			if recorder.Record[i]!=-1{
				avgresponseTime += recorder.Record[i]
				totalNum += 1
			}
		}
		avgresponseTime /= float64(totalNum)

		httpDurationMinutes.WithLabelValues(*appName).Observe(avgresponseTime)
		httpDurationsMinuteHistogram.Observe(avgresponseTime)
		fmt.Println(avgresponseTime)
		GaugeRes200.Set(float64(recorder.Res200Counter)/float64(recorder.Counter))
		GaugeRes5xx.Set(float64(recorder.Res5xxCounter)/float64(recorder.Counter))
		fmt.Println("200:",recorder.Res200Counter)
		fmt.Println("5xx:",recorder.Res5xxCounter)
		recorder.Counter = 0
		recorder.Res200Counter = 0
		recorder.Res5xxCounter = 0
	}


}

//用于获取平均响应时间
func getTime() {
	start := time.Now().UnixNano()
	client := &http.Client{}
	
    //生成要访问的url
	url := "http://139.9.57.167:35520/productpage"
	
    //提交请求
    request, err := http.NewRequest("GET", url, nil)
    if err != nil {
		end := time.Now().UnixNano()
		fmt.Println(start/ int64(time.Millisecond),err)
		flashRecord(start,end,true)
        return
    }
    //处理返回结果
    response ,err := client.Do(request)
    if err != nil {
		end := time.Now().UnixNano()
		fmt.Println(start/ int64(time.Millisecond),err)
		flashRecord(start,end,true)
		return
    }
	end := time.Now().UnixNano()
	//对于5xx和200计数器进行更新
	if response.StatusCode == 200{
		recorder.Res200Counter += 1
	} else if response.StatusCode/100 == 5{
		recorder.Res5xxCounter += 1
	}

	//time即为响应时间
	flashRecord(start,end,false)

}

func init() {
	// Register the summary and the histogram with Prometheus's default registry.
	prometheus.MustRegister(httpDurationSeconds)
	prometheus.MustRegister(httpDurationMinutes)
	prometheus.MustRegister(httpDurationsSecondHistogram)
	prometheus.MustRegister(httpDurationsMinuteHistogram)
	prometheus.MustRegister(GaugeRes200)
	prometheus.MustRegister(GaugeRes5xx)
	// Add Go module build info.
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
	recorder.Counter = 0
}

func main() {
	flag.Parse()
	//计时器初始化
	recorder.Counter = 0
	recorder.Res200Counter = 0
	recorder.Res5xxCounter = 0
	// Periodically record some sample latencies for the three services.
	go func() {
		//计时器，每秒钟计时
		d := time.Duration(time.Second * 1)
		t := time.NewTimer(d)
		defer t.Stop()
		//死循环
		for {

			<- t.C
			go getTime()
			t.Reset(time.Second * 1)
		}
	}()


	// Expose the registered metrics via HTTP.
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))
}

