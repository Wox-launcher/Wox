import { useEffect } from "react"
import { WoxMessageHelper } from "../utils/WoxMessageHelper.ts"
import { WoxMessageMethodEnum } from "../enums/WoxMessageMethodEnum.ts"

export default () => {

  useEffect(() => {
    setTimeout(() => {
      console.log("send action")
      WoxMessageHelper.getInstance().sendMessage(WoxMessageMethodEnum.ACTION.code, {
        "resultId": "1",
        "actionId": "2"
      })
    }, 5000)
  }, [])

  return <div>WoxSetting</div>
}