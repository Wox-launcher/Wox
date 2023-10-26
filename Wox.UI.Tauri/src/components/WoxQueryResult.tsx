import styled from "styled-components"
import { WOXMESSAGE } from "../entity/WoxMessage.typings"
import React, { useImperativeHandle, useRef, useState } from "react"
import { appWindow, LogicalSize } from "@tauri-apps/api/window"

export type WoxQueryResultRefHandler = {
  clearResultList: () => void
  changeResultList: (preview: boolean, results: WOXMESSAGE.WoxMessageResponseResult[]) => void
}

export type WoxQueryResultProps = {}

export default React.forwardRef((_props: WoxQueryResultProps, ref: React.Ref<WoxQueryResultRefHandler>) => {
  const currentWindowHeight = useRef(60)
  const currentResultList = useRef<WOXMESSAGE.WoxMessageResponseResult[]>([])
  const [activeIndex, setActiveIndex] = useState<number>(0)
  const [resultList, setResultList] = useState<WOXMESSAGE.WoxMessageResponseResult[]>([])
  const [hasPreview, setHasPreview] = useState<boolean>(false)


  const resetResultRelatedData = (preview: boolean) => {
    setResultList([])
    setHasPreview(preview)
    setActiveIndex(0)
  }

  const resetResultList = (rsList: WOXMESSAGE.WoxMessageResponseResult[]) => {
    currentResultList.current = [...rsList]
    setResultList(currentResultList.current)
  }

  useImperativeHandle(ref, () => ({
    clearResultList: () => {
      resetResultRelatedData(false)
    },
    changeResultList: (preview: boolean, results: WOXMESSAGE.WoxMessageResponseResult[]) => {
      resetResultRelatedData(preview)
      //reset window size
      if (results.length > 10 || (preview && currentWindowHeight.current === 560)) {
        resetResultList(results)
      } else {
        const windowHeight = hasPreview ? 560 : 60 + 50 * results.length
        appWindow.setSize(new LogicalSize(800, windowHeight)).then(_ => {
          resetResultList(results)
        })
      }
    }
  }))

  return <Style className={"wox-results"}>
    {resultList?.length > 0 && <ul key={"wox-result-list"}>
      {resultList.map((result, index) => {
        return <li key={index}>
          <div className={"wox-result-title"}>{result.Title}</div>
          <div className={"wox-result-subtitle"}>{result.SubTitle}</div>
        </li>
      })}
    </ul>}
  </Style>
})

const Style = styled.div`
  display: flex;
  flex-direction: row;
  overflow-y: auto;

  ul {
    padding: 0;
    margin: 0;
    border-top: 1px solid #dedede;
    max-height: 333px;
    overflow: hidden;
    width: 50%;
  }

  ul:last-child {
    width: 100%;
  }

  ul + div {
    width: 50%;
  }

  ul li {
    display: block;
    height: 45px;
    line-height: 45px;
    border-bottom: 1px solid #dedede;
    -webkit-app-region: no-drag;
    cursor: pointer;
    width: 100%;
  }

  ul li .icon {
    text-align: center;
    line-height: 30px;
    height: 30px;
    width: 30px;
    margin: 7.5px;
    float: left;
  }

  ul li h2,
  ul li h3 {
    margin: 0;
    padding-left: 10px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    font-weight: 400;
    font-family: "Lucida Sans Unicode", "Lucida Grande", sans-serif;
  }

  ul li h2 {
    font-size: 20px;
    line-height: 25px;
  }

  ul li h2:last-child {
    font-size: 20px;
    line-height: 45px;
  }

  ul li h3 {
    font-size: 15px;
    line-height: 15px;
  }

  ul li.active,
  ul li:hover {
    background-color: #dedede;
  }
`