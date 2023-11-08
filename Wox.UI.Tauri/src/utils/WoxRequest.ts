import type { AxiosInstance, AxiosRequestConfig, AxiosResponse } from "axios"
import axios from "axios"
import store from "store2"

type Result<T> = {
  code: number;
  message: string;
  result: T;
};

export class Request {
  instance: AxiosInstance
  baseConfig: AxiosRequestConfig = { baseURL: `http://localhost:${store.get("serverPort")}`, timeout: 60000 }

  constructor(config: AxiosRequestConfig) {
    this.instance = axios.create(Object.assign(this.baseConfig, config))

    this.instance.interceptors.response.use(
      (res: AxiosResponse) => {
        return res.data
      },
      (err: any) => {
        switch (err.response.status) {
          case 400:
            break
          case 401:
            break
          case 403:
            break
          case 404:
            break
          case 408:
            break
          case 500:
            break
          case 501:
            break
          case 502:
            break
          case 503:
            break
          case 504:
            break
          case 505:
            break
          default:
        }
        return Promise.reject(err.response.data)
      }
    )
  }

  public request(config: AxiosRequestConfig): Promise<AxiosResponse> {
    return this.instance.request(config)
  }

  public get<T = any>(
    url: string,
    config?: AxiosRequestConfig
  ): Promise<AxiosResponse<Result<T>>> {
    return this.instance.get(url, config)
  }

  public post<T = any>(
    url: string,
    data?: any,
    config?: AxiosRequestConfig
  ): Promise<AxiosResponse<Result<T>>> {
    return this.instance.post(url, data, config)
  }

  public put<T = any>(
    url: string,
    data?: any,
    config?: AxiosRequestConfig
  ): Promise<AxiosResponse<Result<T>>> {
    return this.instance.put(url, data, config)
  }

  public delete<T = any>(
    url: string,
    config?: AxiosRequestConfig
  ): Promise<AxiosResponse<Result<T>>> {
    return this.instance.delete(url, config)
  }
}

export default new Request({})