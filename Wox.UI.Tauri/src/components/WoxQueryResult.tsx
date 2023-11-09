import styled from "styled-components"
import { WOXMESSAGE } from "../entity/WoxMessage.typings"
import React, { useImperativeHandle, useRef, useState } from "react"
import { WoxTauriHelper } from "../utils/WoxTauriHelper.ts"
import { WoxMessageHelper } from "../utils/WoxMessageHelper.ts"
import { WoxMessageMethodEnum } from "../enums/WoxMessageMethodEnum.ts"
import { WoxMessageRequestMethod, WoxMessageRequestMethodEnum } from "../enums/WoxMessageRequestMethodEnum.ts"
import { WoxPreviewTypeEnum } from "../enums/WoxPreviewTypeEnum.ts"
import Markdown from "react-markdown"
import { Scrollbars } from "react-custom-scrollbars"
import { pinyin } from "pinyin-pro"
import WoxImage from "./WoxImage.tsx"
import { Theme } from "../entity/Theme.typings"
import { WoxThemeHelper } from "../utils/WoxThemeHelper.ts"
import { WOX_QUERY_BOX_INPUT_HEIGHT, WOX_QUERY_RESULT_ITEM_HEIGHT } from "../utils/WoxConst.ts"

export type WoxQueryResultRefHandler = {
  clearResultList: () => void
  changeResultList: (preview: boolean, results: WOXMESSAGE.WoxMessageResponseResult[]) => void
  moveUp: () => void
  moveDown: () => void
  doAction: () => void
  resetMouseIndex: () => void
  toggleActionList: () => Promise<boolean>
  hideActionList: () => void
  isActionListShown: () => boolean
  forceResizeWindow: () => void
}

export type WoxQueryResultProps = {
  callback?: (method: WoxMessageRequestMethod) => void
}

export default React.forwardRef((_props: WoxQueryResultProps, ref: React.Ref<WoxQueryResultRefHandler>) => {
  const currentWindowHeight = useRef(60)
  const currentResultList = useRef<WOXMESSAGE.WoxMessageResponseResult[]>([])
  const currentActionList = useRef<WOXMESSAGE.WoxResultAction[]>([])
  const currentActiveIndex = useRef(0)
  const currentActionActiveIndex = useRef(0)
  const currentMouseOverIndex = useRef(0)
  const currentULRef = useRef<Scrollbars>(null)
  const currentResult = useRef<WOXMESSAGE.WoxMessageResponseResult>()
  const currentFilterText = useRef<string>("")
  const currentPreview = useRef(false)
  const [activeIndex, setActiveIndex] = useState<number>(0)
  const [actionActiveIndex, setActionActiveIndex] = useState<number>(0)
  const [resultList, setResultList] = useState<WOXMESSAGE.WoxMessageResponseResult[]>([])
  const [hasPreview, setHasPreview] = useState<boolean>(false)
  const [actionList, setActionList] = useState<WOXMESSAGE.WoxResultAction[]>([])
  const [showActionList, setShowActionList] = useState<boolean>(false)
  const filterInputRef = React.createRef<HTMLInputElement>()

  const resetResultList = (rsList: WOXMESSAGE.WoxMessageResponseResult[]) => {
    currentActiveIndex.current = 0
    setActiveIndex(0)
    currentResultList.current = [...rsList]
    setResultList(currentResultList.current)
  }

  const getResultSingleItemHeight = () => {
    const theme = WoxThemeHelper.getInstance().getTheme()
    return WOX_QUERY_RESULT_ITEM_HEIGHT + theme.ResultItemPaddingTop + theme.ResultItemPaddingBottom
  }

  const getResultItemHeight = (resultItemCount: number) => {
    const theme = WoxThemeHelper.getInstance().getTheme()
    const baseItemHeight = getResultSingleItemHeight()
    return (currentPreview.current ? baseItemHeight * 10 : baseItemHeight * (resultItemCount > 10 ? 10 : resultItemCount)) + theme.ResultContainerPaddingTop + theme.ResultContainerPaddingBottom
  }

  const getWindowsHeight = (resultItemCount: number) => {
    const theme = WoxThemeHelper.getInstance().getTheme()
    const windowHeight = WOX_QUERY_BOX_INPUT_HEIGHT + theme.AppPaddingTop + theme.AppPaddingBottom
    if (resultItemCount > 0) {
      return windowHeight + getResultItemHeight(resultItemCount)
    }
    return windowHeight
  }

  const resizeWindow = async (resultItemCount: number) => {
    const windowHeight = getWindowsHeight(resultItemCount)
    if (windowHeight > currentWindowHeight.current) {
      currentWindowHeight.current = windowHeight
      return WoxTauriHelper.getInstance().setSize(WoxTauriHelper.getInstance().getWoxWindowWidth(), windowHeight)
    } else {
      currentWindowHeight.current = windowHeight
      return WoxTauriHelper.getInstance().setSize(WoxTauriHelper.getInstance().getWoxWindowWidth(), windowHeight)
    }
  }

  const filterActionList = () => {
    if (currentActionList.current.length > 1) {
      const filteredActionList = currentActionList.current.filter((action) => {
        if (!/[^\u4e00-\u9fa5]/.test(action.Name)) {
          const pyTransfer = pinyin(action.Name)
          return pyTransfer.indexOf(currentFilterText.current) > -1
        }
        return action.Name.toLowerCase().indexOf(currentFilterText.current.toLowerCase()) >= 0

      })
      setActionList(filteredActionList)
      currentActionActiveIndex.current = 0
      setActionActiveIndex(0)
    }
  }

  const sendActionMessage = async (resultId: string, action: WOXMESSAGE.WoxResultAction) => {
    await WoxMessageHelper.getInstance().sendMessage(WoxMessageMethodEnum.ACTION.code, {
      "resultId": resultId,
      "actionId": action.Id
    })
    if (!action.PreventHideAfterAction) {
      _props.callback?.(WoxMessageRequestMethodEnum.HideApp.code)
    }
  }

  const handleAction = async () => {
    if (showActionList) {
      const result = currentResultList.current.find((result) => result.Index === currentActiveIndex.current)
      if (result) {
        currentResult.current = result
        if (currentActionActiveIndex.current < actionList.length) {
          const action = actionList[currentActionActiveIndex.current]
          if (action) {
            await sendActionMessage(result.Id, action)
          }
        }
      }
    } else {
      const result = currentResultList.current.find((result) => result.Index === currentActiveIndex.current)
      if (result) {
        currentResult.current = result
        for (const action of result.Actions) {
          if (action.IsDefault) {
            await sendActionMessage(result.Id, action)
          }
        }
      }
    }
  }

  const getCurrentPreviewData = () => {
    const result = currentResultList.current.find((result) => result.Index === currentActiveIndex.current)
    if (result) {
      return result.Preview
    }
    return { PreviewType: "", PreviewData: "", PreviewProperties: {} } as WOXMESSAGE.WoxPreview
  }

  const handleMoveUp = () => {
    if (showActionList) {
      currentActionActiveIndex.current = actionActiveIndex <= 0 ? actionList.length - 1 : actionActiveIndex - 1
      setActionActiveIndex(currentActionActiveIndex.current)
    } else {
      currentMouseOverIndex.current = 0
      currentActiveIndex.current = currentActiveIndex.current <= 0 ? currentResultList.current.length - 1 : currentActiveIndex.current - 1
      setActiveIndex(currentActiveIndex.current)
      if (currentActiveIndex.current >= 10) {
        currentULRef.current?.scrollTop(getResultSingleItemHeight() * (currentActiveIndex.current - 9))
      }
      if (currentActiveIndex.current === currentResultList.current.length - 1) {
        currentULRef.current?.scrollTop(getResultSingleItemHeight() * (currentResultList.current.length - 1))
      }
    }
  }

  const handleMoveDown = () => {
    if (showActionList) {
      currentActionActiveIndex.current = actionActiveIndex >= actionList.length - 1 ? 0 : actionActiveIndex + 1
      setActionActiveIndex(currentActionActiveIndex.current)
    } else {
      currentMouseOverIndex.current = 0
      currentActiveIndex.current = currentActiveIndex.current >= currentResultList.current.length - 1 ? 0 : currentActiveIndex.current + 1
      setActiveIndex(currentActiveIndex.current)
      if (currentActiveIndex.current >= 10) {
        currentULRef.current?.scrollTop(getResultSingleItemHeight() * (currentActiveIndex.current - 9))
      }
      if (currentActiveIndex.current === 0) {
        currentULRef.current?.scrollTop(0)
      }
    }
  }

  const handleHideActionList = async () => {
    setShowActionList(false)
    setActionActiveIndex(0)
    currentMouseOverIndex.current = 0
    await resizeWindow(currentResultList.current.length)
  }

  const handleToggleActionList = async () => {
    if (showActionList) {
      await handleHideActionList()
      return false
    } else {
      const result = currentResultList.current.find((result) => result.Index === currentActiveIndex.current)
      if (result) {
        currentResult.current = result
        resizeWindow(10).then(_ => {
          currentActionList.current = result.Actions
          setActionList(result.Actions)
          setShowActionList(true)
        })
      }
      return true
    }
  }

  useImperativeHandle(ref, () => ({
    clearResultList: () => {
      setActiveIndex(0)
      resizeWindow(0)
    },
    changeResultList: (preview: boolean, results: WOXMESSAGE.WoxMessageResponseResult[]) => {
      currentPreview.current = preview
      setHasPreview(preview)
      if (currentWindowHeight.current === getWindowsHeight(results.length)) {
        resetResultList(results)
      } else {
        resizeWindow(results.length).then(_ => {
          resetResultList(results)
        })
      }
    },
    moveUp: () => {
      handleMoveUp()
    },
    moveDown: () => {
      handleMoveDown()
    },
    doAction: () => {
      handleAction()
    },
    resetMouseIndex: () => {
      setShowActionList(false)
      currentMouseOverIndex.current = 0
    },
    toggleActionList: async () => {
      return await handleToggleActionList()
    },
    hideActionList: () => {
      handleHideActionList()
    },
    isActionListShown: () => {
      return showActionList
    },
    forceResizeWindow: () => {
      resizeWindow(currentResultList.current.length)
    }
  }))

  return <Style theme={WoxThemeHelper.getInstance().getTheme()} resultCount={resultList.length} itemHeight={getResultItemHeight(10)}>
    <Scrollbars
      className={"wox-result-scrollbars"}
      style={{ width: hasPreview ? "50%" : "100%" }}
      ref={currentULRef}
      autoHeight
      autoHeightMin={0}
      autoHeightMax={getResultItemHeight(resultList.length < 10 ? 10 : resultList.length)}>
      <div className={"wox-result-container"}>
        <ul id={"wox-result-list"} key={"wox-result-list"}>
          {resultList.map((result, index) => {
            return <li id={`wox-result-li-${index}`} key={`wox-result-li-${index}`} className={activeIndex === index ? "active" : "inactive"}
                       onMouseOverCapture={() => {
                         if (showActionList) {
                           return
                         }
                         currentMouseOverIndex.current += 1
                         if (result.Index !== undefined && currentActiveIndex.current !== result.Index && currentMouseOverIndex.current > 1) {
                           currentActiveIndex.current = index
                           setActiveIndex(index)
                         }
                       }}
                       onClick={(event) => {
                         handleAction()
                         event.preventDefault()
                         event.stopPropagation()
                       }}>
              <WoxImage img={result.Icon} height={36} width={36} />
              <h2 className={"wox-result-title"}>{result.Title}</h2>
              {result.SubTitle && <h3 className={"wox-result-subtitle"}>{result.SubTitle}</h3>}
            </li>
          })}
        </ul>
      </div>
    </Scrollbars>


    {hasPreview && getCurrentPreviewData().PreviewType !== "" &&
      <div
        className={"wox-query-result-preview"}>
        <Scrollbars
          autoHeight
          autoHeightMin={0}
          autoHeightMax={getResultSingleItemHeight() * 8 + 10}>
          <div className={"wox-query-result-preview-content"}>
            {getCurrentPreviewData().PreviewType === WoxPreviewTypeEnum.WoxPreviewTypeText.code && <p>{getCurrentPreviewData().PreviewData}</p>}
            {getCurrentPreviewData().PreviewType === WoxPreviewTypeEnum.WoxPreviewTypeImage.code &&
              <img src={getCurrentPreviewData().PreviewData}
                   className={"wox-query-result-preview-image"} />}
            {getCurrentPreviewData().PreviewType === WoxPreviewTypeEnum.WoxPreviewTypeImage.code &&
              <Markdown>{getCurrentPreviewData().PreviewData}</Markdown>}
            {getCurrentPreviewData().PreviewType === WoxPreviewTypeEnum.WoxPreviewTypeUrl.code &&
              <iframe src={getCurrentPreviewData().PreviewData} className={"wox-query-result-preview-url"}></iframe>}
          </div>
        </Scrollbars>
        {Object.keys(getCurrentPreviewData().PreviewProperties)?.length > 0 &&
          <div className={"wox-query-result-preview-properties"}>
            {Object.keys(getCurrentPreviewData().PreviewProperties)?.map((key) => {
              return <div key={`key-${key}`} className={"wox-query-result-preview-property"}>
                <div
                  className={"wox-query-result-preview-property-key"}>{key}</div>
                <div
                  className={"wox-query-result-preview-property-value"}>{getCurrentPreviewData().PreviewProperties[key]}</div>
              </div>
            })}
          </div>
        }
      </div>}

    {showActionList && <div className={"wox-query-result-action-list"} onClick={(event) => {
      event.preventDefault()
      event.stopPropagation()
    }}>
      <div className={"wox-query-result-action-list-header"}>Actions</div>
      {actionList.map((action, index) => {
        return <div key={`wox-result-action-item-${index}`}
                    className={index === actionActiveIndex ? "wox-result-action-item wox-result-action-item-active" : "wox-result-action-item"}
                    onClick={(event) => {
                      sendActionMessage(currentResult.current?.Id || "", action)
                      event.preventDefault()
                      event.stopPropagation()
                    }}>
          <WoxImage img={action.Icon} width={24} height={24} />
          <span className={"wox-result-action-item-name"}>{action.Name}</span>
        </div>
      })}
      <div className={"wox-action-list-filter"}>
        <input ref={filterInputRef} className={"wox-action-list-filter-input mousetrap"} type="text"
               aria-label="Wox"
               autoComplete="off"
               autoCorrect="off"
               autoCapitalize="off"
               autoFocus={true}
               onChange={(e) => {
                 currentFilterText.current = e.target.value
                 filterActionList()
               }} />
      </div>
    </div>
    }
  </Style>
})

const Style = styled.div<{ theme: Theme, resultCount: number, itemHeight: number }>`
  display: flex;
  flex-direction: row;
  min-height: ${props => props.itemHeight}px;

  .wox-result-scrollbars {
    flex: 1;
  }

  .wox-result-container {
    padding-top: ${props => props.theme.ResultContainerPaddingTop}px;
    padding-right: ${props => props.theme.ResultContainerPaddingRight}px;
    padding-bottom: ${props => props.theme.ResultContainerPaddingBottom}px;
    padding-left: ${props => props.theme.ResultContainerPaddingLeft}px;
  }

  ul {
    padding: 0;
    margin: 0;
    overflow: hidden;
    width: 100%;
  }

  ul + div {
    width: 50%;
  }

  ul li {
    display: block;
    height: 50px;
    line-height: 50px;
    cursor: pointer;
    width: 100%;
    box-sizing: border-box;
    border-radius: ${props => props.theme.ResultItemBorderRadius}px;
    padding-top: ${props => props.theme.ResultItemPaddingTop}px;
    padding-right: ${props => props.theme.ResultItemPaddingRight}px;
    padding-bottom: ${props => props.theme.ResultItemPaddingBottom}px;
    padding-left: ${props => props.theme.ResultItemPaddingLeft}px;
  }

  ul li:last-child {
    margin-bottom: 3px;
  }

  ul li .wox-image {
    text-align: center;
    margin: 7px;
    float: left;
  }

  ul li h2,
  ul li h3 {
    margin: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    line-height: 30px;
  }

  ul li h2 {
    font-size: 18px;
    font-weight: 550;
    color: ${props => props.theme.ResultItemTitleColor};
  }

  ul li h2:last-child {
    line-height: 50px;
  }

  ul li h3 {
    font-size: 13px;
    line-height: 15px;
    font-weight: normal;
    color: ${props => props.theme.ResultItemSubTitleColor};
  }

  ul li.active {
    background-color: ${props => props.theme.ResultItemActiveBackgroundColor};
  }

  .wox-query-result-preview {
    flex: 1;
    position: relative;
    box-sizing: border-box;
    width: 50%;
    display: inline-block;
    border-left: 1px solid ${props => props.theme.PreviewSplitLineColor};
    padding: 10px 0 10px 10px;
    color: ${props => props.theme.PreviewFontColor};

    .wox-query-result-preview-content {
      overflow: hidden;
      position: relative;

      p {
        word-wrap: break-word;
      }

      .wox-query-result-preview-image {
        width: 100%;
        max-height: 400px;
      }

      .wox-query-result-preview-url {
        width: 100%;
        height: 400px;
      }
    }

    .wox-query-result-preview-properties {
      position: absolute;
      left: 0;
      bottom: 0;
      right: 0;
      overflow: hidden;

      .wox-query-result-preview-property {
        display: flex;
        justify-content: space-between;
        width: 100%;
        border-top: 1px solid ${props => props.theme.PreviewSplitLineColor};
        padding: 2px 10px;
        overflow: hidden;

        .wox-query-result-preview-property-key {
          font-weight: 500;
          color: ${props => props.theme.PreviewPropertyTitleColor};
        }

        .wox-query-result-preview-property-value {
        color: ${props => props.theme.PreviewPropertyContentColor};
        }

        .div {
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
      }
    }
  }

  .wox-query-result-action-list {
    position: absolute;
    bottom: 10px;
    right: 10px;
    background-color: ${props => props.theme.ActionContainerBackgroundColor};
    min-width: 300px;
    padding-left: ${props => props.theme.ActionContainerPaddingLeft}px;
    padding-right: ${props => props.theme.ActionContainerPaddingRight}px;
    padding-top: ${props => props.theme.ActionContainerPaddingTop}px;
    padding-bottom: ${props => props.theme.ActionContainerPaddingBottom}px;
    z-index: 9999;

    .wox-query-result-action-list-header {
      color: ${props => props.theme.ActionContainerHeaderFontColor};
    }

    .wox-result-action-item {
      display: flex;
      line-height: 30px;
      align-items: center;
      padding: 5px 10px;

      .wox-image {
        margin-right: 8px;
      }
    }

    .wox-result-action-item-active {
      background-color: ${props => props.theme.ActionItemActiveBackgroundColor};
    }

    .wox-action-list-filter {
      margin-top: 5px;

      .wox-action-list-filter-input {
        width: 100%;
        box-sizing: border-box;
        height: 32px;
        line-height: 32px;
        font-size: 18px;
        outline: none;
        border: 0;
        padding: 0 8px;
        cursor: auto;
        color: ${props => props.theme.ActionQueryBoxFontColor};
        display: inline-block;
        background-color: ${props => props.theme.ActionQueryBoxBackgroundColor};
        border-radius: ${props => props.theme.ActionQueryBoxBorderRadius}px;
      }
    }
  }
`
