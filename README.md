# oss_go
使用go语言实现的文件存储服务
列出目录内容
请求

方法：POST
路径：/list
请求体：
json
Copy code
{
    "path": "/example"
}
path (可选): 要列出的目录路径，如果不提供，默认为根目录。
响应

状态码：200 OK
响应体：
json
Copy code
{
    "status": 1,
    "message": "success",
    "content": [
        {
            "name": "example",
            "is_dir": true,
            "date": "2022-12-01T16:34:24Z"
        },
        {
            "name": "file.txt",
            "is_dir": false,
            "date": "2022-12-01T16:44:14Z"
        }
    ]
}
上传文件
请求

方法：POST
路径：/upload
请求体：
json
Copy code
{
    "path": "/example/uploaded_file.txt"
}
path: 要上传的文件路径。
响应

状态码：200 OK
响应体：
json
Copy code
{
    "status": 1,
    "message": "文件上传成功",
    "filePath": "/example/uploaded_file.txt"
}
删除文件或目录
请求

方法：POST
路径：/delete
请求体：
json
Copy code
{
    "path": "/example/file_to_delete.txt"
}
path: 要删除的文件或目录路径。
响应

状态码：200 OK
响应体：
json
Copy code
{
    "status": 1,
    "message": "删除成功"
}
获取文件
请求

方法：GET
路径：/get/example/file_to_get.txt
响应

状态码：200 OK
响应头：
Content-Type: application/octet-stream
Content-Disposition: attachment; filename=file_to_get.txt
响应体：文件内容
