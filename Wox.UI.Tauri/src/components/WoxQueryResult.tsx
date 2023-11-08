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
import { WoxThemeHelper } from "../utils/WoxThemeHelper.ts"

export type WoxQueryResultRefHandler = {
  clearResultList: () => void
  changeResultList: (preview: boolean, results: WOXMESSAGE.WoxMessageResponseResult[]) => void
  moveUp: () => void
  moveDown: () => void
  doAction: () => void
  resetMouseIndex: () => void
  toggleActionList: () => void
  hideActionList: () => void
  isActionListShown: () => boolean
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

  const resizeWindowAndResultList = (results: WOXMESSAGE.WoxMessageResponseResult[], windowHeight: number) => {
    if (windowHeight > currentWindowHeight.current) {
      WoxTauriHelper.getInstance().setSize(WoxTauriHelper.getInstance().getWoxWindowWidth(), windowHeight).then(_ => {
        resetResultList(results)
      })
    } else {
      resetResultList(results)
      WoxTauriHelper.getInstance().setSize(WoxTauriHelper.getInstance().getWoxWindowWidth(), windowHeight)
    }
    currentWindowHeight.current = windowHeight
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
        currentULRef.current?.scrollTop(50 * (currentActiveIndex.current - 9))
      }
      if (currentActiveIndex.current === currentResultList.current.length - 1) {
        currentULRef.current?.scrollTop(50 * (currentResultList.current.length - 1))
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
        currentULRef.current?.scrollTop(50 * (currentActiveIndex.current - 9))
      }
      if (currentActiveIndex.current === 0) {
        currentULRef.current?.scrollTop(0)
      }
    }
  }

  const handleHideActionList = () => {
    setShowActionList(false)
    setActionActiveIndex(0)
    currentMouseOverIndex.current = 0
    const windowHeight = hasPreview ? 560 : 60 + 50 * (currentResultList.current.length > 10 ? 10 : currentResultList.current.length)
    WoxTauriHelper.getInstance().setSize(WoxTauriHelper.getInstance().getWoxWindowWidth(), windowHeight)
  }

  const handleToggleActionList = async () => {
    if (showActionList) {
      handleHideActionList()
    } else {
      const result = currentResultList.current.find((result) => result.Index === currentActiveIndex.current)
      if (result) {
        currentResult.current = result
        //resize window
        currentWindowHeight.current = 560
        WoxTauriHelper.getInstance().setSize(WoxTauriHelper.getInstance().getWoxWindowWidth(), currentWindowHeight.current).then(_ => {
          currentActionList.current = result.Actions
          setActionList(result.Actions)
          setShowActionList(true)
        })
      }
    }
  }

  useImperativeHandle(ref, () => ({
    clearResultList: () => {
      setActiveIndex(0)
      resizeWindowAndResultList([], 60)
    },
    changeResultList: (preview: boolean, results: WOXMESSAGE.WoxMessageResponseResult[]) => {
      setHasPreview(preview)
      //reset window size
      const windowHeight = preview ? 560 : 60 + 50 * (results.length > 10 ? 10 : results.length)
      if (currentWindowHeight.current === windowHeight) {
        resetResultList(results)
      } else {
        resizeWindowAndResultList(results, windowHeight)
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
    toggleActionList: () => {
      handleToggleActionList()
    },
    hideActionList: () => {
      handleHideActionList()
    },
    isActionListShown: () => {
      return showActionList
    }
  }))

  return <Style className={resultList.length > 0 ? "wox-results wox-result-border" : "wox-results"}>
    <Scrollbars
      style={{ width: hasPreview ? 400 : 800 }}
      ref={currentULRef}
      autoHeight
      autoHeightMin={0}
      autoHeightMax={500}>
      <ul id={"wox-result-list"} key={"wox-result-list"}>
        {resultList.map((result, index) => {
          return <li id={`wox-result-li-${index}`} key={`wox-result-li-${index}`} className={activeIndex === index ? "active" : "inactive"} onMouseOverCapture={() => {
            currentMouseOverIndex.current += 1
            if (result.Index !== undefined && currentActiveIndex.current !== result.Index && currentMouseOverIndex.current > 1) {
              currentActiveIndex.current = index
              setActiveIndex(index)
            }
          }} onClick={(event) => {
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
    </Scrollbars>


    {hasPreview && getCurrentPreviewData().PreviewType !== "" &&
      <div
        className={"wox-query-result-preview"}>
        <Scrollbars
          autoHeight
          autoHeightMin={0}
          autoHeightMax={410}>
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

    {showActionList && <div className={"wox-query-result-action-container"} onClick={() => {
      setShowActionList(false)
    }}>
      <div className={"wox-query-result-action-list"} onClick={(event) => {
        event.preventDefault()
        event.stopPropagation()
      }}>
        <div className={"wox-query-result-action-list-header"}>Actions</div>
        {actionList.map((action, index) => {
          return <div key={`wox-result-action-item-${index}`}
                      className={index === actionActiveIndex ? "wox-result-action-item wox-result-action-item-active" : "wox-result-action-item"} onClick={(event) => {
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
    </div>}
  </Style>
})

const Style = styled.div`
  display: flex;
  flex-direction: row;
  width: 800px;
  border-bottom: ${WoxTauriHelper.getInstance().isTauri() ? "0px" : "1px"} solid #dedede;


  ul {
    padding: 0;
    margin: 0;
    overflow: hidden;
    width: 50%;
    border-right: ${WoxTauriHelper.getInstance().isTauri() ? "0px" : "1px"} solid #dedede;;
  }

  ul:last-child {
    width: 100%;
  }

  ul + div {
    width: 50%;
  }

  ul li {
    display: block;
    height: 50px;
    line-height: 50px;
    border-bottom: 1px solid #dedede;
    cursor: pointer;
    width: 100%;
    box-sizing: border-box;
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
    font-weight: 400;
    font-family: "Lucida Sans Unicode", "Lucida Grande", sans-serif;
  }

  ul li h2 {
    font-size: 20px;
    line-height: 30px;
  }

  ul li h2:last-child {
    font-size: 20px;
    line-height: 50px;
  }

  ul li h3 {
    font-size: 13px;
    line-height: 15px;
  }

  // ul li.active {
    //   background-color: ${WoxThemeHelper.getInstance().getTheme().ResultActiveBackgroundColor};
  // }

  .wox-query-result-preview {
    position: relative;
    width: 400px;
    min-height: 490px;
    border-left: 1px solid #dedede;
    padding: 10px 0 10px 10px;
    border-right: ${WoxTauriHelper.getInstance().isTauri() ? "0px" : "1px"} solid #dedede;

    .wox-query-result-preview-content {
      max-height: 400px;
      overflow-y: auto;

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
      max-height: 90px;
      overflow-y: auto;

      .wox-query-result-preview-property {
        display: flex;
        width: 100%;
        border-top: 1px solid #dee2e6;
        padding: 2px 10px;
        overflow: hidden;

        .wox-query-result-preview-property-key {
          flex: 3;
        }

        .wox-query-result-preview-property-value {
          flex: 4;
        }

        .wox-query-result-preview-property-key, .wox-query-result-preview-property-value {
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
      }
    }
  }

  .wox-query-result-action-container {
    position: absolute;
    left: 0;
    right: 0;
    top: 60px;
    bottom: 0;
    z-index: 8888;
  }

  .wox-query-result-action-list {
    position: absolute;
    bottom: 10px;
    right: 10px;
    background-color: #e8e8e6;
    border: 1px solid #dedede;
    min-width: 300px;
    padding: 5px 10px;
    z-index: 9999;

    .wox-query-result-action-list-header {
      color: #747473;
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
      background-color: #d1d1cf;
    }

    .wox-action-list-filter {
      border-top: 1px solid #dedede;
      padding-top: 5px;
      margin-top: 5px;

      .wox-action-list-filter-input {
        width: 100%;
        font-size: 18px;
        outline: none;
        border: 0;
        padding: 0 5px;
        cursor: auto;
        color: black;
        display: inline-block;
        background-color: transparent;
      }
    }
  }
`
