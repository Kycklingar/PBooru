package benchmark

import (
	"fmt"
	"time"
)

type timer struct {
	start time.Time
	split []split
}

type split struct {
	key  string
	time time.Time
}

func Begin() timer {
	var t timer
	t.Start()
	return t
}

func (t *timer) Start() {
	t.start = time.Now()
}

func (t *timer) Split(str string) {
	t.split = append(t.split, split{key: str, time: time.Now()})
}

func (t *timer) End(p bool) time.Duration {
	t.Split("End")
	if p {
		fmt.Println("-Benchmark began:", t.start.Format("15:04:05.00000"))
		for _, s := range t.split {
			_, err := fmt.Println(s.key, s.time.Local().Sub(t.start).Seconds())
			if err != nil {
				fmt.Println(err)
			}
		}
		fmt.Println("-Benchmark ended:", t.split[len(t.split)-1].time.Format("15:04:05.00000"))
	}
	end := t.split[len(t.split)-1].time.Local().Sub(t.start)

	return end
}

func (t *timer) EndStr(p bool) string {
	return fmt.Sprint(float64(int64(float64(t.End(p).Seconds()*2000+0.5))) / 2000)
}
