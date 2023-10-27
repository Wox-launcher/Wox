import styled from "styled-components"
import { WOXMESSAGE } from "../entity/WoxMessage.typings"
import React, { useImperativeHandle, useRef, useState } from "react"
import { appWindow, LogicalSize } from "@tauri-apps/api/window"
import { WoxImageTypeEnum } from "../enums/WoxImageTypeEnum.ts"

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
      const windowHeight = preview ? 560 : 60 + 50 * (results.length > 10 ? 10 : results.length)
      if (currentWindowHeight.current === windowHeight) {
        resetResultList(results)
      } else {
        currentWindowHeight.current = windowHeight
        appWindow.setSize(new LogicalSize(800, windowHeight)).then(_ => {
          resetResultList(results)
        })
      }
    }
  }))

  return <Style className={"wox-results"}>
    {resultList?.length > 0 && <ul key={"wox-result-list"}>
      {resultList.map((result, index) => {
        return <li key={index} className={activeIndex === index ? "active" : "inactive"}>
          {result.Icon.ImageType === WoxImageTypeEnum.WoxImageTypeSvg.code &&
            <div className={"wox-query-result-image"}
                 dangerouslySetInnerHTML={{ __html: result.Icon.ImageData }}></div>}
          {result.Icon.ImageType === WoxImageTypeEnum.WoxImageTypeUrl.code &&
            <img src={result.Icon.ImageData} className={"wox-query-result-image"} alt={"query-result-image"} />}
          {result.Icon.ImageType === WoxImageTypeEnum.WoxImageTypeBase64.code &&
            <img src={result.Icon.ImageData} className={"wox-query-result-image"} alt={"query-result-image"} />}
          <h2 className={"wox-result-title"}>{result.Title}</h2>
          <h3 className={"wox-result-subtitle"}>{result.SubTitle}</h3>
        </li>
      })}
    </ul>}
  </Style>
})

const Style = styled.div`
  display: flex;
  flex-direction: row;
  overflow-y: auto;
  height: 100%;

  ul {
    padding: 0;
    margin: 0;
    max-height: 500px;
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
    height: 50px;
    line-height: 50px;
    border-bottom: 1px solid #dedede;
    -webkit-app-region: no-drag;
    cursor: pointer;
    width: 100%;
  }

  ul li .wox-query-result-image {
    text-align: center;
    line-height: 36px;
    height: 36px;
    width: 36px;
    margin: 7px;
    float: left;

    svg {
      width: 36px !important;
      height: 36px !important;
    }
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
    line-height: 30px;
  }

  ul li h3 {
    font-size: 13px;
    line-height: 15px;
  }

  ul li.active,
  ul li:hover {
    background-color: #dedede;
  }
`