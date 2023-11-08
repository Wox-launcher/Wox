import axios, { AxiosRequestConfig, AxiosResponse } from "axios"
import { WoxMessageHelper } from "./WoxMessageHelper.ts"

const instance = axios.create({
  timeout: 10000
})

function getBaseUrl() {
  return `http://127.0.0.1:${WoxMessageHelper.getInstance().getPort()}`
}

export function request<T = any>(
  config: AxiosRequestConfig
): Promise<AxiosResponse<T>> {
  return instance.request(config)
}

export function get<T = any>(
  url: string,
  config?: AxiosRequestConfig
): Promise<AxiosResponse<T>> {
  url = getBaseUrl() + url
  return instance.get(url, config)
}

export function post<T = any>(
  url: string,
  data?: any,
  config?: AxiosRequestConfig
): Promise<AxiosResponse<T>> {
  url = getBaseUrl() + url
  return instance.post(url, data, config)
}

export function put<T = any>(
  url: string,
  data?: any,
  config?: AxiosRequestConfig
): Promise<AxiosResponse<T>> {
  url = getBaseUrl() + url
  return instance.put(url, data, config)
}

export function del<T = any>(
  url: string,
  config?: AxiosRequestConfig
): Promise<AxiosResponse<T>> {
  url = getBaseUrl() + url
  return instance.delete(url, config)
}