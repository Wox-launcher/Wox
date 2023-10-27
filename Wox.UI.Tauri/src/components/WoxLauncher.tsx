import styled from "styled-components"
import WoxQueryBox, { WoxQueryBoxRefHandler } from "./WoxQueryBox.tsx"
import React, { useEffect, useRef } from "react"
import WoxQueryResult, { WoxQueryResultRefHandler } from "./WoxQueryResult.tsx"
import { WOXMESSAGE } from "../entity/WoxMessage.typings"
import { WoxMessageHelper } from "../utils/WoxMessageHelper.ts"
import { appWindow, LogicalPosition } from "@tauri-apps/api/window"
import { WoxPositionTypeEnum } from "../enums/WoxPositionTypeEnum.ts"
import { hide, show } from "@tauri-apps/api/app"
import { WoxMessageRequestMethodEnum } from "../enums/WoxMessageRequestMethodEnum.ts"
import { useInterval } from "usehooks-ts"
import { WoxMessageMethodEnum } from "../enums/WoxMessageMethodEnum.ts"
import { WoxLogHelper } from "../utils/WoxLogHelper.ts"
import { WoxTauriHelper } from "../utils/WoxTauriHelper.ts"

export default () => {
  const woxQueryBoxRef = React.useRef<WoxQueryBoxRefHandler>(null)
  const woxQueryResultRef = React.useRef<WoxQueryResultRefHandler>(null)
  const requestTimeoutId = useRef<number>()
  const refreshTotalCount = useRef<number>(0)
  const hasLatestQueryResult = useRef<boolean>(true)
  const currentQuery = useRef<string>()
  const fullResultList = useRef<WOXMESSAGE.WoxMessageResponseResult[]>([])

  /*
    Deal with query change event
   */
  const onQueryChange = (query: string) => {
    currentQuery.current = query
    fullResultList.current = []
    clearTimeout(requestTimeoutId.current)
    hasLatestQueryResult.current = false
    WoxMessageHelper.getInstance().sendQueryMessage({
      query: query,
      type: "text"
    }, handleQueryCallback)
    requestTimeoutId.current = setTimeout(() => {
      if (!hasLatestQueryResult.current) {
        woxQueryResultRef.current?.clearResultList()
      }
    }, 500)
  }

  const refreshResults = async () => {
    let needUpdate = false
    let preview = false
    const currentCount = refreshTotalCount.current
    for (const [i, result] of fullResultList.current.entries()) {
      if (result.RefreshInterval > 0) {
        if (currentCount % result.RefreshInterval === 0) {
          const refreshableResult = {
            Title: result.Title,
            SubTitle: result.SubTitle,
            Icon: result.Icon,
            Preview: result.Preview,
            ContextData: result.ContextData,
            RefreshInterval: result.RefreshInterval
          } as WOXMESSAGE.WoxRefreshableResult

          let response = await WoxMessageHelper.getInstance().sendMessage(WoxMessageMethodEnum.REFRESH.code, {
            "resultId": result.Id,
            "refreshableResult": JSON.stringify(refreshableResult)
          })
          if (response.Success) {
            const newResult = response.Data as WOXMESSAGE.WoxRefreshableResult
            if (newResult) {
              fullResultList.current[i].Title = newResult.Title
              fullResultList.current[i].SubTitle = newResult.SubTitle
              fullResultList.current[i].Icon = newResult.Icon
              fullResultList.current[i].Preview = newResult.Preview
              fullResultList.current[i].ContextData = newResult.ContextData
              fullResultList.current[i].RefreshInterval = newResult.RefreshInterval
              preview = !!newResult.Preview.PreviewType
              needUpdate = true
            }
          } else {
            WoxLogHelper.getInstance().log(`refresh [${result.Title}] failed: ${response.Data}`)
          }
        }
      }
    }

    if (needUpdate) {
      woxQueryResultRef.current?.changeResultList(preview, [...fullResultList.current])
    }
  }

  /*
    Because the query callback will be called multiple times, so we need to filter the result by query text
   */
  const handleQueryCallback = (results: WOXMESSAGE.WoxMessageResponseResult[]) => {
    let preview = false
    fullResultList.current = fullResultList.current.concat(results.filter((result) => {
      if (result.AssociatedQuery === currentQuery.current) {
        hasLatestQueryResult.current = true
      }
      return result.AssociatedQuery === currentQuery.current
    })).map((result, index) => {
      preview = !!result.Preview.PreviewType
      return Object.assign({ ...result, Id: index, Index: index })
    })

    //sort fullResultList order by score desc
    fullResultList.current.sort((a, b) => {
      return b.Score - a.Score
    })
    woxQueryResultRef.current?.changeResultList(preview, [...fullResultList.current])
  }

  /*
    Deal with global request event
   */
  const handleRequestCallback = async (message: WOXMESSAGE.WoxMessage) => {
    if (message.Method === WoxMessageRequestMethodEnum.ChangeQuery.code) {
      await changeQuery(message.Data as string)
    }
    if (message.Method === WoxMessageRequestMethodEnum.HideApp.code) {
      await hideWoxWindow()
    }
    if (message.Method === WoxMessageRequestMethodEnum.ShowApp.code) {
      await showWoxWindow(message.Data as WOXMESSAGE.ShowContext)
    }
    if (message.Method === WoxMessageRequestMethodEnum.ToggleApp.code) {
      appWindow.isVisible().then(async visible => {
        if (visible) {
          await hideWoxWindow()
        } else {
          await showWoxWindow(message.Data as WOXMESSAGE.ShowContext)
        }
      })
    }
  }

  /*
    Show wox window
   */
  const showWoxWindow = async (context: WOXMESSAGE.ShowContext) => {
    if (context.Position.Type === WoxPositionTypeEnum.WoxPositionTypeMouseScreen.code) {
      await appWindow.setPosition(new LogicalPosition(Number(context.Position.X), Number(context.Position.Y)))
    }
    if (context.SelectAll) {
      woxQueryBoxRef.current?.selectAll()
    }
    await appWindow.setFocus()
    await show()
  }

  /*
    Hide wox window
   */
  const hideWoxWindow = async () => {
    await hide()
  }

  const changeQuery = async (query: string) => {
    woxQueryBoxRef.current?.changeQuery(query)
  }

  useInterval(async () => {
    refreshTotalCount.current = refreshTotalCount.current + 100
    await refreshResults()
  }, 100)


  useEffect(() => {
    WoxTauriHelper.getInstance().setFocus().then(_ => {
      WoxMessageHelper.getInstance().initialRequestCallback(handleRequestCallback)
    })
  }, [])

  return <Style className={"wox-launcher"} onKeyDown={(event) => {
    if (event.key === "Escape") {
      hideWoxWindow()
      event.preventDefault()
      event.stopPropagation()
    }
  }}>
    <WoxQueryBox ref={woxQueryBoxRef} onQueryChange={onQueryChange} />
    <WoxQueryResult ref={woxQueryResultRef} />
  </Style>
}

const Style = styled.div`
  overflow: hidden;
  display: flex;
  flex-direction: column;
`