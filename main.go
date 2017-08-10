package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
)

var (
	appdir = flag.String("dir", "app.d", "app dir")
)

type App struct {
	Name string
	Cmd  *exec.Cmd
}

func (a *App) log(line string) {
	fmt.Printf("[%s] %s\n", a.Name, line)
}

func (a *App) readAndLog(r io.Reader) {
	rd := bufio.NewScanner(r)
	for rd.Scan() {
		a.log(rd.Text())
	}
}

func (a *App) Start(exitch chan struct{}, wg *sync.WaitGroup) error {
	stdout, _ := a.Cmd.StdoutPipe()
	stderr, _ := a.Cmd.StderrPipe()
	err := a.Cmd.Start()
	if err != nil {
		exitch <- struct{}{}
		wg.Done()
		return err
	}
	a.log("started")

	go a.readAndLog(stdout)
	go a.readAndLog(stderr)
	go func() {
		err = a.Cmd.Wait()
		if err != nil {
			a.log(err.Error())
		}
		a.log("stoped")
		exitch <- struct{}{}
		wg.Done()
	}()

	return nil
}

func (a *App) Kill() {
	a.Cmd.Process.Kill()
}

func scanapps(dir string) ([]*App, error) {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var apps []*App

	for _, info := range infos {
		if filepath.Ext(info.Name()) != ".sh" {
			continue
		}
		name := filepath.Join(dir, info.Name())
		cmd := exec.Command("bash", "-c", name)
		app := &App{
			Name: strings.TrimSuffix(info.Name(), filepath.Ext(info.Name())),
			Cmd:  cmd,
		}
		apps = append(apps, app)
	}

	return apps, nil
}

func main() {
	flag.Parse()

	apps, err := scanapps(*appdir)
	if err != nil {
		log.Fatal(err)
	}

	wg := new(sync.WaitGroup)
	wg.Add(len(apps))
	exitch := make(chan struct{}, len(apps))
	for _, app := range apps {
		app.Start(exitch, wg)
	}

	intch := make(chan os.Signal)
	signal.Notify(intch, os.Interrupt, os.Kill)

	select {
	case <-exitch:
	case <-intch:
	}

	for _, app := range apps {
		app.Kill()
	}

	wg.Wait()
}
