package main

import (
	"context"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	configFile = "sync.yml"
)

type config struct {
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
	Errors      string `yaml:"errors"`
	Threads     int    `yaml:"threads"`
	MailsFile   string `yaml:"mails_file"`
}

type mail struct {
	login1 string
	pass1  string
	login2 string
	pass2  string
}

type mails []mail

func main() {
	conf, err := readConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}
	fileName := conf.MailsFile
	if len(os.Args) > 1 {
		fileName = os.Args[1]
	}
	mm, err := readFile(fileName)
	if err != nil {
		log.Fatal(err)
	}
	start := time.Now()
	conf.runSync(context.Background(), mm)
	finish := time.Now().Sub(start).Minutes()
	log.Printf("Синхронизация почтовых ящиков завершена, общее время - %0.f мин.\n", finish)
}

func readConfig(fn string) (*config, error) {
	buf, err := os.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	conf := &config{}
	err = yaml.Unmarshal(buf, conf)
	return conf, err
}

func readFile(fn string) (mails, error) {
	buf, err := os.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(buf), "\n")
	mm := make(mails, 0, len(lines))
	for _, line := range lines {
		fields := strings.Split(line, ";")
		if len(fields) < 2 {
			continue
		}
		m := mail{
			login1: fields[0],
			pass1:  fields[1],
			login2: fields[0],
			pass2:  fields[1],
		}
		if len(fields) > 2 {
			m.login2 = fields[2]
		}
		if len(fields) > 3 {
			m.pass2 = fields[3]
		}
		mm = append(mm, m)
	}
	return mm, nil
}

func (c config) runSync(ctx context.Context, mm mails) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	mailsCh := make(chan mail)
	increment := make(chan int)
	go func() {
		for i := 1; i <= len(mm); i++ {
			increment <- i
		}
	}()
	wg := sync.WaitGroup{}
	wg.Add(len(mm))
	go func() {}()
	for i := 0; i < c.Threads; i++ {
		go func() {
			for {
				select {
				case m := <-mailsCh:
					cmd := exec.Command("./imapsync",
						"--host1", c.Source, "--user1", m.login1, "--password1", m.pass1,
						"--host2", c.Destination, "--user2", m.login2, "--password2", m.pass2,
						"--nolog", "--errorsmax", c.Errors)
					num := <-increment
					log.Printf("%d. Начало обработки почты: %s\n", num, m.login2)
					start := time.Now()
					err := cmd.Run()
					finish := time.Now().Sub(start).Minutes()
					if err != nil {
						log.Printf("%d. Ошибка обработки почты %s (%0.f мин): %v\n", num, m.login2, finish, err)
					} else {
						log.Printf("%d. Конец обработки почты: %s (%0.f мин)\n", num, m.login2, finish)
					}
					wg.Done()
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	for _, m := range mm {
		mailsCh <- m
	}
	wg.Wait()
}
