#推送服务


## 配置参数
```
addr: 0.0.0.0:8080         #监听端口
password: abc123           #access 密码, 业务服务器获取access_key的密码
```


### 流程
```
角色
    1. 客户端
        一般指页面js
    2. 业务端
        业务服务器， 比如app后端
    3. 推送服务
        即本服务
流程
0. 用户在业务服务器上登录
1. 客户端 向 业务服务器 获取 access_key
    1.1 业务服务器 请求 推送服务 的 [/access] 接口, 参数见下面详情
    1.2 推送服务 返回 access_key 到 业务服务器
    1.3 业务服务器 返回 access_key 到 客户端
2. 客户端 使用 access_key 来长连接到 订阅[/ws] 接口, 来接受推送消息

3. 业务服务器 需要 推送消息时 请求 [/push] 接口， 推送到uid对应的 客户端
```
    


##1. 订阅接口

页面上js连接此接口，用来接收服务器推送消息
```
调用方: 页面js
协议: websocket
地址: /ws?uid=#uid&access_key=#access_key
参数：
    1. uid          用户唯一标识符
    2. access_key   订阅access_key 通过服务器获取
    
推送消息格式(onmessage  e.data)：
    {
        "topic":"",             #/push接口传入的topic
        "number":1,
        "contents":{            #/push接口传入的content
            "abc":"asdfasd"
        } 
    }
```


example:
```js
let ws = new WebSocket("ws://#host/ws?uid=#uid&access_key=#access_key");
ws.onopen = function(e){
    report("系统", "连接");
}
ws.onmessage = function(e){
    let json = JSON.parse(e.data);
    //当客户端收到服务端发来的消息时，触发onmessage事件，参数e.data包含server传递过来的数据
    console.log(`form-server:${e.data}`);
    report(json.topic, json.contents, "success");
}
```

##2. 推送接口

业务服务器 推送消息 到客户端
```
调用方: 业务服务器
协议: http post
地址: /push?uid=#uid&password=#password
header: [Content-type: application/json]
消息体: {
    topic: string   消息主题        必填
    contents: any   消息内容    必填
}
参数：
  url中:
    1. uid          用户唯一标识符
    2. access_key   订阅access_key 通过服务器获取
  body中:
    1. topic        消息主题 string
    2. contents     消息内容 string|number|array|object 任意类型
    
返回：
    正确：
    {
        "topic":"success",
        "number":1,
        "contents":"推送成功"
    }
    错误:
    {
        "topic": "error",
        "contents": "",     #错误说明 :此用户不在线
    }
```


example:
```php
//场景， 会员资料更新后
function httpPostRaw($url, $data){
    $headers = array(
        "Content-type: application/json;charset='utf-8'"
    );
    $ch = curl_init($url);
    curl_setopt($ch, CURLOPT_TIMEOUT, 60); //设置超时
    $url = '这里为请求地址';
    if(0 === strpos(strtolower($url), 'https')) {
        curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, 0); //对认证证书来源的检查
        curl_setopt($ch, CURLOPT_SSL_VERIFYHOST, 0); //从证书中检查SSL加密算法是否存在
    }
    curl_setopt($ch, CURLOPT_POST, TRUE);
    curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode($data)); 
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, TRUE); 
    curl_setopt($ch, CURLOPT_HTTPHEADER, $headers); 
    $rtn = curl_exec($ch);//CURLOPT_RETURNTRANSFER 不设置  curl_exec返回TRUE 设置  curl_exec返回json(此处) 失败都返回FALSE
    curl_close($ch);

    return $rtn;
}
$url = "http://localhost:8080/push?uid=#uid&password=#password";
$data = [
    'topic' => "info_update",
    "contents" => [
        'name' => "张三",
        'age' => 17
    ]
];
httpPostRaw($url, $data)
```

##3. 获取access_key

客户端订阅长连接， 需要access_key, 通过 业务服务器中转推送服务 获取 access_key， 从而解决安全问题
```
调用方: 业务服务器
协议: http
地址: /access?uid=${uid}&password=123123
参数：
    1. uid          用户唯一标识符
    2. access_key   订阅access_key 通过服务器获取
    
返回：
    正确：
    {
        "topic": "_access",
        "contents": "",     #access_key    
    }
    错误:
    {
        "topic": "error",
        "contents": "",     #错误说明  
    }
```

example:
```php
$url = "http://{$domain}/access?uid={$uid}&password={$password}";
$msg = json_decode(file_get_contents($url), true);
if($msg['topic'] == "_access"){
    $access_key = $msg['contents']
}
```

# 部署
1. 服务器上安装 go
2. 运行 go build ./main.go -o push-serve
3. chmod o+x push-serve
4. ./push-serve

#补充
1. 业务服务器 需要保留 推送服务器 一样的 密码， 密码在 conf.yaml[password] 中
2. 客户端不能保存 password ， 而且 access_key 要通过业务服务器来获取， 从而提供安全性
    客户端直接用password 来获取access_key 的行为要绝对禁止， 否则， 客户端可以不受任何限制的接受任意会员的推送消息
3. 业务服务器 提供 access_key 时， 需要对接口限制为已登录用户
4. 水平有限， 服务最好一段时间重启一次