package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	// 检查当前目录下是否有 data 目录
	_, err := os.Stat("data")
	if os.IsNotExist(err) {
		// 不存在，创建 data 目录
		err := os.MkdirAll("data", os.ModePerm)
		if err != nil {
			log.Printf("Error: 无法创建 data 目录 %s\n", err)
		}
	} else if err != nil {
		// 其他错误
		log.Printf("Error: 无法获取 data 目录信息 %s\n", err)
	}
	http.HandleFunc("/get/", getFileHandler)

	// 读取配置文件中的 token
	config, err := LoadConfig()
	if err != nil {
		log.Printf("Error loading config: %s\n", err)
		return
	}

	// 如果需要拦截的接口，应用 TokenMiddleware 中间件
	http.Handle("/list", TokenMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		listHandler(w, r)
	}), config.Token))

	http.Handle("/upload", TokenMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadHandler(w, r)
	}), config.Token))

	http.Handle("/delete", TokenMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deleteHandler(w, r)
	}), config.Token))

	err = http.ListenAndServe("0.0.0.0:8082", nil)
	if err != nil {
		log.Printf("Error: 服务启动失败 %s\n", err)
	}
}

// Config 结构用于解析配置文件中的 JSON 数据
type Config struct {
	Token string `json:"token"`
}

// LoadConfig 从配置文件中加载配置信息
func LoadConfig() (Config, error) {
	var config Config

	// 读取配置文件
	data, err := os.ReadFile("config.json")
	if err != nil {
		return config, err
	}

	// 解析 JSON 数据
	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}

// TokenMiddleware 是用于检查请求头中 token 的中间件
func TokenMiddleware(next http.Handler, validToken string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从请求头中获取 token
		token := r.Header.Get("Authorization")

		// 检查 token 是否有效
		if token != validToken {
			// 返回错误响应
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// 如果 token 有效，调用下一个处理程序
		next.ServeHTTP(w, r)
	})
}

// 获取文件
func getFileHandler(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Path[len("/get/"):]
	fullPath := filepath.Join("data", filePath)

	// 检查路径是否是文件夹
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，记录日志并返回 JSON 提示未找到
			sendJSONResponse(w, http.StatusNotFound, "资源文件不存在", err, r.URL.Path)
			return
		}
		// 其他错误，记录日志并返回 JSON 提示服务器错误
		sendJSONResponse(w, http.StatusInternalServerError, "服务器错误，请稍后重试", err, r.URL.Path)
		return
	}

	if fileInfo.IsDir() {
		// 如果是文件夹，记录日志并返回 JSON 提示未找到
		sendJSONResponse(w, http.StatusNotFound, "资源文件不存在", err, r.URL.Path)
		return
	}

	// 如果是文件，将文件流式返回
	file, err := os.Open(fullPath)
	if err != nil {
		// 文件打开失败，记录日志并返回 JSON 提示服务器错误
		sendJSONResponse(w, http.StatusInternalServerError, "服务器错误，请稍后重试", err, r.URL.Path)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Error: closing file %s\n", err)
		}
	}(file)

	// 设置响应头
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileInfo.Name()))

	// 将文件内容写入响应
	http.ServeContent(w, r, fileInfo.Name(), fileInfo.ModTime(), file)
	log.Printf("info: %s \n", r.URL.Path)
}

// 获取上传的文件并存储
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// 获取存储路径
	path := r.Header.Get("X-FormFile-Path")
	if path == "" {
		sendJSONResponse(w, http.StatusBadRequest, "缺少存储路径", nil, r.URL.Path)
		return
	}

	// 获取上传的文件
	file, _, err := r.FormFile("file")
	if err != nil {
		sendJSONResponse(w, http.StatusBadRequest, "接收文件失败", err, "")
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Error: closing file %s\n", err)
		}
	}(file)

	// 获取目录部分
	dir := filepath.Dir(path)

	// 根据文件名生成存储路径
	fullPath := filepath.Join("data", dir)

	// 检查目录是否存在，不存在则创建
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		err := os.MkdirAll(fullPath, os.ModePerm)
		if err != nil {
			sendJSONResponse(w, http.StatusInternalServerError, "创建目录失败", err, r.URL.Path)
			return
		}
	}

	// 创建文件
	newFilePath := filepath.Join(fullPath, filepath.Base(path))
	newFile, err := os.Create(newFilePath)
	if err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, "创建文件失败", err, r.URL.Path)
		return
	}
	defer func(newFile *os.File) {
		err := newFile.Close()
		if err != nil {
			log.Printf("Error: closing file %s\n", err)
		}
	}(newFile)

	// 将上传的文件内容复制到新文件
	_, err = io.Copy(newFile, file)
	if err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, "文件复制失败", err, r.URL.Path)
		return
	}

	sendJSONResponse(w, http.StatusOK, "文件上传成功", nil, r.URL.Path)
	log.Printf("info: %s \n", r.URL.Path)
}

// ListRequest 结构用于解析列出目录的请求的 JSON 数据
type ListRequest struct {
	Path string `json:"path"`
}

// ListResponse 结构用于组织列出目录的响应
type ListResponse struct {
	Status  int         `json:"status"`
	Message string      `json:"message"`
	Content []ListEntry `json:"content"`
}

// ListEntry 结构用于表示目录中的文件或文件夹信息
type ListEntry struct {
	Name  string    `json:"name"`
	IsDir bool      `json:"is_dir"`
	Date  time.Time `json:"date"`
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	// 解析 JSON 请求体
	var listRequest ListRequest
	err := json.NewDecoder(r.Body).Decode(&listRequest)
	if err != nil {
		sendListResponse(w, http.StatusBadRequest, "缺少必要参数", ListResponse{
			Status:  0,
			Content: []ListEntry{},
		}, err, r.URL.Path)
		return
	}

	// 获取 path 参数
	path := listRequest.Path

	// 如果 path 为空，则列出 data 目录下的文件和文件夹
	if path == "" {
		path = "data"
	} else {
		path = "data/" + path
	}

	// 获取完整路径
	fullPath := filepath.Join(path)

	// 检查目录是否存在
	_, err = os.Stat(fullPath)
	if err != nil {
		sendListResponse(w, http.StatusOK, "该目录不存在", ListResponse{
			Status:  0,
			Content: []ListEntry{},
		}, err, r.URL.Path)
		return
	}

	// 列出目录内容
	entries, err := listDirectory(fullPath)
	if err != nil {
		sendListResponse(w, http.StatusInternalServerError, "无法列出目录内容", ListResponse{
			Status:  0,
			Content: []ListEntry{},
		}, err, r.URL.Path)
		return
	}

	// 构建响应
	response := ListResponse{
		Status:  1,
		Message: "success",
		Content: entries,
	}

	// 发送响应
	sendListResponse(w, http.StatusOK, "success", response, err, r.URL.Path)
	log.Printf("info: %s \n", r.URL.Path)
}

func listDirectory(path string) ([]ListEntry, error) {
	var entries []ListEntry

	// 打开目录
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func(dir *os.File) {
		err := dir.Close()
		if err != nil {
			log.Printf("Error: closing file %s\n", err)
		}
	}(dir)

	// 读取目录内容
	fileInfos, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}

	// 遍历文件和文件夹
	for _, fileInfo := range fileInfos {
		entry := ListEntry{
			Name:  fileInfo.Name(),
			IsDir: fileInfo.IsDir(),
			Date:  fileInfo.ModTime(),
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func sendListResponse(w http.ResponseWriter, statusCode int, message string, response ListResponse, err error, url string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response.Message = message
	if err != nil {
		log.Printf("Error: %s %s\n", err, url)
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("Error: %s\n", err)
		return
	}
}

// sendJSONResponse 发送 JSON 格式的响应
func sendJSONResponse(w http.ResponseWriter, statusCode int, message string, err error, url string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if statusCode == 200 {
		response := map[string]interface{}{
			"status":  1,
			"message": message,
		}
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			log.Printf("Error: %s %s\n", err, url)
			return
		}
		return
	}

	response := map[string]interface{}{
		"status":  0,
		"message": message,
	}

	if err != nil {
		log.Printf("Error: %s %s\n", err, url)
	} else {
		log.Printf("Error: %s %s\n", message, url)
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("Error: %s %s\n", err, url)
		return
	}
}

// DeleteRequest 结构用于解析删除请求的 JSON 数据
type DeleteRequest struct {
	Path string `json:"path"`
}

// DeleteResponse 结构用于组织删除响应
type DeleteResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	// 解析 JSON 请求体
	var deleteRequest DeleteRequest
	err := json.NewDecoder(r.Body).Decode(&deleteRequest)
	if err != nil {
		sendDeleteResponse(w, http.StatusBadRequest, DeleteResponse{
			Status:  0,
			Message: "缺少必要参数",
		}, err, r.URL.Path)
		return
	}

	// 获取 path 参数
	path := deleteRequest.Path

	// 如果 path 为空，则返回错误
	if path == "" {
		sendDeleteResponse(w, http.StatusBadRequest, DeleteResponse{
			Status:  0,
			Message: "缺少路径参数",
		}, err, r.URL.Path)
		return
	}

	// 获取完整路径
	fullPath := filepath.Join("data", path)

	// 检查文件或目录是否存在
	_, err = os.Stat(fullPath)
	if os.IsNotExist(err) {
		sendDeleteResponse(w, http.StatusOK, DeleteResponse{
			Status:  0,
			Message: "文件或目录不存在",
		}, err, r.URL.Path)
		return
	} else if err != nil {
		sendDeleteResponse(w, http.StatusInternalServerError, DeleteResponse{
			Status:  0,
			Message: "无法获取文件或目录信息",
		}, err, r.URL.Path)
		return
	}

	// 删除文件或目录
	err = os.RemoveAll(fullPath)
	if err != nil {
		sendDeleteResponse(w, http.StatusInternalServerError, DeleteResponse{
			Status:  0,
			Message: "删除失败",
		}, err, r.URL.Path)
		return
	}

	// 构建响应
	response := DeleteResponse{
		Status:  1,
		Message: "删除成功",
	}

	// 发送响应
	sendDeleteResponse(w, http.StatusOK, response, nil, r.URL.Path)
	log.Printf("info: %s \n", r.URL.Path)
}

func sendDeleteResponse(w http.ResponseWriter, statusCode int, response DeleteResponse, err error, url string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err != nil {
		log.Printf("Error: %s %s\n", err, url)
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("Error: %s\n", err)
		return
	}
}
