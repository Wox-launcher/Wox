import styled from "styled-components"
import WoxQueryBox, { WoxQueryBoxRefHandler } from "./WoxQueryBox.tsx"
import React, { useEffect, useRef } from "react"
import WoxQueryResult, { WoxQueryResultRefHandler } from "./WoxQueryResult.tsx"
import { WOXMESSAGE } from "../entity/WoxMessage.typings"
import { WoxMessageHelper } from "../utils/WoxMessageHelper.ts"
import { WoxMessageRequestMethod, WoxMessageRequestMethodEnum } from "../enums/WoxMessageRequestMethodEnum.ts"
import { useInterval } from "usehooks-ts"
import { WoxMessageMethodEnum } from "../enums/WoxMessageMethodEnum.ts"
import { WoxLogHelper } from "../utils/WoxLogHelper.ts"
import { WoxTauriHelper } from "../utils/WoxTauriHelper.ts"
import Mousetrap from "mousetrap"

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
    woxQueryResultRef.current?.hideActionList()
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
    }, 200)
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
    fullResultList.current = fullResultList.current.concat(results.filter((result) => {
      if (result.AssociatedQuery === currentQuery.current) {
        hasLatestQueryResult.current = true
      }
      return result.AssociatedQuery === currentQuery.current
    }))

    //sort fullResultList order by score desc
    fullResultList.current.sort((a, b) => {
      return b.Score - a.Score
    })

    let preview = false
    fullResultList.current = fullResultList.current.map((result, index) => {
      preview = !!result.Preview.PreviewType
      return Object.assign({ ...result, Index: index })
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
  }

  /*
    Hide wox window
   */
  const hideWoxWindow = async () => {
    await WoxTauriHelper.getInstance().hideWindow()
    await WoxMessageHelper.getInstance().sendMessage(WoxMessageMethodEnum.ON_VISIBILITY_CHANGED.code, {
      "isVisible": "false",
      "query": currentQuery.current || ""
    })
  }

  /*
    Change query text
   */
  const changeQuery = async (query: string) => {
    woxQueryBoxRef.current?.changeQuery(query)
  }

  const bindKeyboardEvent = () => {
    Mousetrap.bind("esc", (event) => {
      woxQueryResultRef.current?.resetMouseIndex()
      hideWoxWindow()
      event.preventDefault()
      event.stopPropagation()
    })
    Mousetrap.bind("down", (event) => {
      woxQueryResultRef.current?.moveDown()
      event.preventDefault()
      event.stopPropagation()
    })
    Mousetrap.bind("up", (event) => {
      woxQueryResultRef.current?.moveUp()
      event.preventDefault()
      event.stopPropagation()
    })
    Mousetrap.bind("enter", (event) => {
      woxQueryResultRef.current?.doAction()
      event.preventDefault()
      event.stopPropagation()
    })
    Mousetrap.bind("command+j", (event) => {
      woxQueryResultRef.current?.showActionList()
      event.preventDefault()
      event.stopPropagation()
    })
  }

  useInterval(async () => {
    refreshTotalCount.current = refreshTotalCount.current + 100
    await refreshResults()
  }, 100)


  useEffect(() => {
    WoxTauriHelper.getInstance().setFocus()
    WoxMessageHelper.getInstance().initialRequestCallback(handleRequestCallback)
    bindKeyboardEvent()

    // @ts-ignore expose to tauri backend
    window.selectAll = () => {
      woxQueryBoxRef.current?.selectAll()
      woxQueryResultRef.current?.resetMouseIndex()
    }
    // @ts-ignore expose to tauri backend
    window.focus = () => {
      woxQueryBoxRef.current?.focus()
    }
  }, [])

  return <Style className={"wox-launcher"}>
    <WoxQueryBox ref={woxQueryBoxRef} onQueryChange={onQueryChange} onFocus={() => {
      woxQueryResultRef.current?.hideActionList()
    }} />

    <WoxQueryResult ref={woxQueryResultRef} callback={(method: WoxMessageRequestMethod) => {
      if (method === WoxMessageRequestMethodEnum.HideApp.code) {
        hideWoxWindow()
      }
    }} />
  </Style>
}

const Style = styled.div`
  overflow: hidden;
  display: flex;
  flex-direction: column;

  .wox-result-border {
    border-top: 1px solid #dedede !important;
  }
`