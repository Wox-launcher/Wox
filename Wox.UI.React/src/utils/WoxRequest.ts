import axios, { AxiosRequestConfig, AxiosResponse } from "axios"
import { WoxMessageHelper } from "./WoxMessageHelper.ts"

const instance = axios.create({
  timeout: 10000
})

function getBaseUrl() {
  return `http://127.0.0.1:${WoxMessageHelper.getInstance().getPort()}`
}

export async function request<T = any>(
  config: AxiosRequestConfig
): Promise<AxiosResponse<T>> {
  const response = await instance.request(config)
  return response.data
}

export async function get<T = any>(
  url: string,
  config?: AxiosRequestConfig
): Promise<T> {
  url = getBaseUrl() + url
  const response = await instance.get(url, config)
  return response.data
}

export async function post<T = any>(
  url: string,
  data?: any,
  config?: AxiosRequestConfig
): Promise<T> {
  url = getBaseUrl() + url
  const response = await instance.post(url, data, config)
  return response.data
}

export async function put<T = any>(
  url: string,
  data?: any,
  config?: AxiosRequestConfig
): Promise<T> {
  url = getBaseUrl() + url
  const response = await instance.put(url, data, config)
  return response.data
}

export async function del<T = any>(
  url: string,
  config?: AxiosRequestConfig
): Promise<T> {
  url = getBaseUrl() + url
  const response = await instance.delete(url, config)
  return response.data
}