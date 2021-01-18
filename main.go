package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"

	//"os"
	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	Addr string `json:"addr"`
	Password string `json:"password"`
}

type Message struct {
	Uid      int         `json:"-"`
	Topic    string      `json:"topic"`
	Number   int         `json:"number"`
	Contents interface{} `json:"contents"`
}
func (msg *Message) ToJson() ([]byte, error){
	return json.Marshal(msg)
}

func simpleMessage(contents interface{}, topic string) *Message {
	message := &Message{}
	message.Topic = topic
	message.Number = 1
	message.Contents = contents
	return message
}

func newMessage(Topic string, Number int, Contents interface{}) *Message {
	message := &Message{}
	message.Topic = Topic
	message.Number = Number
	message.Contents = Contents
	return message
}

func create_md5(s string) string {
	m := md5.Sum([]byte (s))
	return hex.EncodeToString(m[:])
}

func  GetRandomString(l int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

var upgrader = websocket.Upgrader{} // use default options

//已存在的连接
var connections = make(map[string]*websocket.Conn)

//权限
var access_keys = make(map[string]string)


var config = &Config{}

var messageChannels = make(chan Message)

func WsHandle(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	values := r.URL.Query()
	uid := values.Get("uid")
	access_key := values.Get("access_key")
	if uid == "" {
		m := simpleMessage("请填写uid", "error")

		data, _ := json.Marshal(m)
		c.WriteMessage(0, []byte(data))
		return
	}
	if access_keys[uid] != access_key {
		m := simpleMessage("access_key不正确", "error")

		data, _ := json.Marshal(m)
		c.WriteMessage(0, []byte(data))
		return
	}

	connections[uid] = c
	log.Println("login:", uid)
	defer delete(connections, uid)

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			//log.Println("read:", err)
			break
		}
		//log.Println("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			//log.Println("write:", err)
			break
		}
	}
}

func PushHandle(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()

	uid := values.Get("uid")
	password := values.Get("password")
	if uid == "" {
		w.WriteHeader(403)

		m := simpleMessage("推送失败, 请填写接受消息的用户uid", "error")

		data, _ := json.Marshal(m)
		fmt.Fprintf(w, string(data))
		return
	}

	if config.Password != password {
		w.WriteHeader(401)

		m := simpleMessage("推送失败, 没有权限推送", "error")

		data, _ := json.Marshal(m)
		fmt.Fprintf(w, string(data))
		return
	}

	if connections[uid] == nil {
		w.WriteHeader(401)

		m := simpleMessage("此用户不在线", "error")

		data, _ := json.Marshal(m)
		fmt.Fprintf(w, string(data))
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("read body err, %v\n", err)
		return
	}
	log.Println("json:", string(body))

	var a Message
	if err = json.Unmarshal(body, &a); err != nil {
		w.WriteHeader(403)

		m := simpleMessage("推送失败, 数据解析错误", "error")

		data, _ := json.Marshal(m)
		fmt.Fprintf(w, string(data))
		return
	} else {
		//直接推送， todo使用channel推送
		var connection = connections[uid]
		push_data, _ := json.Marshal(newMessage(a.Topic, 1, a.Contents))
		err = connection.WriteMessage(1, []byte(push_data))
		if err != nil {
			log.Println("push-error:", err)
			return
		}
		log.Println("sended:", uid, string(push_data))

		m := simpleMessage("推送成功", "success")
		data, _ := json.Marshal(m)
		fmt.Fprintf(w, string(data))
		return
	}
}

func AccessHandle(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()

	uid := values.Get("uid")
	password := values.Get("password")
	if uid == "" {
		w.WriteHeader(403)

		m := simpleMessage("推送失败, 请填写接受消息的用户uid", "error")
		data, _ := json.Marshal(m)
		fmt.Fprintf(w, string(data))
		return
	}

	//配置了密码， 但是传来的密码不对
	if config.Password != "" && config.Password != password {
		w.WriteHeader(403)

		m := simpleMessage("获取链接参数失败 - 密码错误", "error")
		data, _ := json.Marshal(m)
		fmt.Fprintf(w, string(data))
		return
	}
	access_key := create_md5(uid + password + GetRandomString(5))

	//存入缓存
	access_keys[uid] = access_key

	go func() {
		//60s有效期
		time.Sleep(60 * time.Second)
		delete(access_keys, uid)
		log.Println("access_key:", uid, " deleted!")
	}()

	log.Println(uid, " 获取access_key:", access_key)
	m := simpleMessage(access_key, "_access")
	data, _ := json.Marshal(m)
	fmt.Fprintf(w, string(data))
	return
}

func loadConf(confFile string) *Config{
	c := &Config{}
	yamlFile, err := ioutil.ReadFile(confFile)
	if err != nil {
		log.Println("配置文件读取失败 err   #%v ", err)
		return nil
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
		return nil
	}
	return c
}

func main() {
	config = loadConf("./conf.yaml")
	if config == nil {
		log.Println("配置文件读取失败")
		return
	}
	log.Println("服务开始：", config.Addr)
	//推送api
	http.HandleFunc("/push", PushHandle)
	//安全
	http.HandleFunc("/access", AccessHandle)
	//websocket
	http.HandleFunc("/ws", WsHandle)
	//http.Handle("/", http.FileServer(http.Dir("./pages/")))
	http.ListenAndServe(config.Addr, nil)
}
