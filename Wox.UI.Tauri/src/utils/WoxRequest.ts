import axios, { AxiosRequestConfig, AxiosResponse } from "axios"
import store from "store2"


const instance = axios.create({
  baseURL: `http://127.0.0.1:${store.get("serverPort")}`,
  timeout: 10000
})

export function request<T = any>(
  config: AxiosRequestConfig
): Promise<AxiosResponse<T>> {
  return instance.request(config)
}

export function get<T = any>(
  url: string,
  config?: AxiosRequestConfig
): Promise<AxiosResponse<T>> {
  return instance.get(url, config)
}

export function post<T = any>(
  url: string,
  data?: any,
  config?: AxiosRequestConfig
): Promise<AxiosResponse<T>> {
  return instance.post(url, data, config)
}

export function put<T = any>(
  url: string,
  data?: any,
  config?: AxiosRequestConfig
): Promise<AxiosResponse<T>> {
  return instance.put(url, data, config)
}

export function del<T = any>(
  url: string,
  config?: AxiosRequestConfig
): Promise<AxiosResponse<T>> {
  return instance.delete(url, config)
}