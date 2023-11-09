import styled from "styled-components"
import WoxQueryBox, { WoxQueryBoxRefHandler } from "./WoxQueryBox.tsx"
import React, { useEffect, useReducer, useRef } from "react"
import WoxQueryResult, { WoxQueryResultRefHandler } from "./WoxQueryResult.tsx"
import { WOXMESSAGE } from "../entity/WoxMessage.typings"
import { WoxMessageHelper } from "../utils/WoxMessageHelper.ts"
import { WoxMessageRequestMethod, WoxMessageRequestMethodEnum } from "../enums/WoxMessageRequestMethodEnum.ts"
import { useInterval } from "usehooks-ts"
import { WoxMessageMethodEnum } from "../enums/WoxMessageMethodEnum.ts"
import { WoxLogHelper } from "../utils/WoxLogHelper.ts"
import { WoxTauriHelper } from "../utils/WoxTauriHelper.ts"
import Mousetrap from "mousetrap"
import { WoxThemeHelper } from "../utils/WoxThemeHelper.ts"
import { Theme } from "../entity/Theme.typings"

export default () => {
  const [_, forceUpdate] = useReducer((x) => x + 1, 0)
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
    if (message.Method === WoxMessageRequestMethodEnum.ChangeTheme.code) {
      await changeTheme(message.Data as string)
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

  const changeTheme = async (theme: string) => {
    await WoxThemeHelper.getInstance().changeTheme(JSON.parse(theme) as Theme)
    forceUpdate()
  }

  const bindKeyboardEvent = () => {
    Mousetrap.bind("esc", (event) => {
      if (woxQueryResultRef.current?.isActionListShown()) {
        woxQueryResultRef.current?.hideActionList()
        woxQueryBoxRef.current?.focus()
      } else {
        woxQueryResultRef.current?.resetMouseIndex()
        hideWoxWindow()
      }
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
      woxQueryResultRef.current?.toggleActionList()
      event.preventDefault()
      event.stopPropagation()
    })
    //TODO: for test: 'show setting page'
    Mousetrap.bind("command+t", (event) => {
      WoxTauriHelper.getInstance().openWindow("Wox Setting", "setting.html")
      event.preventDefault()
      event.stopPropagation()
    })
  }

  useInterval(async () => {
    refreshTotalCount.current = refreshTotalCount.current + 100
    await refreshResults()
  }, 100)


  useEffect(() => {
    WoxMessageHelper.getInstance().initialRequestCallback(handleRequestCallback)
    bindKeyboardEvent()

    // @ts-ignore expose to tauri backend
    window.selectAll = () => {
      woxQueryBoxRef.current?.selectAll()
      woxQueryResultRef.current?.resetMouseIndex()
    }
    // @ts-ignore expose to tauri backend
    window.postShow = () => {
      woxQueryBoxRef.current?.focus()
      WoxMessageHelper.getInstance().sendMessage(WoxMessageMethodEnum.ON_VISIBILITY_CHANGED.code, {
        "isVisible": "true",
        "query": woxQueryBoxRef.current?.getQuery() || ""
      })
    }
  }, [])

  return <Style theme={WoxThemeHelper.getInstance().getTheme()} className={"wox-launcher"}>
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

const Style = styled.div<{ theme: Theme }>`
  background-color: ${props => props.theme.AppBackgroundColor};
  padding-top: ${props => props.theme.AppPaddingTop}px;
  padding-right: ${props => props.theme.AppPaddingRight}px;
  padding-bottom: ${props => props.theme.AppPaddingBottom}px;
  padding-left: ${props => props.theme.AppPaddingLeft}px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
`