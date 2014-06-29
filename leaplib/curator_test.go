/*
Copyright (c) 2014 Ashley Jeffs

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package leaplib

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func curatorTestConfig() CuratorConfig {
	conf := DefaultCuratorConfig()
	conf.LoggerConfig.LogLevel = LeapError
	return conf
}

func TestNewCurator(t *testing.T) {
	cur, err := CreateNewCurator(curatorTestConfig())
	if err != nil {
		t.Errorf("Create curator error: %v", err)
		return
	}

	cur.Close()
}

func TestCuratorClients(t *testing.T) {
	config := DefaultBinderConfig()
	config.FlushPeriod = 5000

	curator, err := CreateNewCurator(curatorTestConfig())
	if err != nil {
		t.Errorf("error: %v", err)
		return
	}

	doc, err := CreateNewDocument("test", "test1", "text", "hello world")
	if err != nil {
		t.Errorf("error: %v", err)
		return
	}

	portal, err := curator.NewDocument("", doc)
	doc = portal.Document
	if err != nil {
		t.Errorf("error: %v", err)
	}

	tform := func(i int) OTransform {
		return OTransform{
			Position: 0,
			Version:  i,
			Delete:   0,
			Insert:   fmt.Sprintf("%v", i),
		}
	}

	if v, err := portal.SendTransform(tform(portal.Version+1), time.Second); v != 2 || err != nil {
		t.Errorf("Send Transform error, v: %v, err: %v", v, err)
	}

	wg := sync.WaitGroup{}
	wg.Add(20)

	tformSending := 50

	for i := 0; i < 10; i++ {
		if b, e := curator.FindDocument("", doc.ID); e != nil {
			t.Errorf("error: %v", e)
		} else {
			go goodClient(b, tformSending, t, &wg)
		}
		if b, e := curator.FindDocument("", doc.ID); e != nil {
			t.Errorf("error: %v", e)
		} else {
			go badClient(b, t, &wg)
		}
	}

	wg.Add(50)

	for i := 0; i < 50; i++ {
		if i%2 == 0 {
			if b, e := curator.FindDocument("", doc.ID); e != nil {
				t.Errorf("error: %v", e)
			} else {
				go goodClient(b, tformSending-i, t, &wg)
			}
			if b, e := curator.FindDocument("", doc.ID); e != nil {
				t.Errorf("error: %v", e)
			} else {
				go badClient(b, t, &wg)
			}
		}
		if v, err := portal.SendTransform(tform(i+3), time.Second); v != i+3 || err != nil {
			t.Errorf("Send Transform error, expected v: %v, got v: %v, err: %v", i+3, v, err)
		}
	}

	go func() {
		for {
			select {
			case err := <-curator.errorChan:
				t.Errorf("Curator received error: %v", err)
			case <-time.After(50 * time.Millisecond):
				return
			}
		}
	}()

	closeChan := make(chan bool)

	go func() {
		curator.Close()
		wg.Wait()
		closeChan <- true
	}()

	go func() {
		time.Sleep(1 * time.Second)
		closeChan <- false
	}()

	if closeStatus := <-closeChan; !closeStatus {
		t.Errorf("Timeout occured waiting for test finish.")
	}
}
