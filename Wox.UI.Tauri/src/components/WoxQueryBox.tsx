import {Col, FormControl, Image, InputGroup, ListGroup, Row} from "react-bootstrap";
import React, {useEffect, useRef, useState} from "react";
import {WoxMessageHelper} from "../utils/WoxMessageHelper.ts";
import styled from "styled-components";
import {WOXMESSAGE} from "../entity/WoxMessage.typings";
import {WoxMessageMethodEnum} from "../enums/WoxMessageMethodEnum.ts";
import {WoxImageTypeEnum} from "../enums/WoxImageTypeEnum.ts";
import {WoxPreviewTypeEnum} from "../enums/WoxPreviewTypeEnum.ts";
import Markdown from 'react-markdown'
import {hide, show} from '@tauri-apps/api/app';
import {WoxMessageRequestMethodEnum} from "../enums/WoxMessageRequestMethodEnum.ts";
import {appWindow, LogicalSize} from "@tauri-apps/api/window";

const queryBoxRef = React.createRef<
    HTMLInputElement
>();
export default () => {
    const queryText = useRef<string>()
    const currentResultList = useRef<WOXMESSAGE.WoxMessageResponseResult[]>([])
    const [resultList, setResultList] = useState<WOXMESSAGE.WoxMessageResponseResult[]>([])
    const [activeIndex, setActiveIndex] = useState<number>(0)
    const currentIndex = useRef(0)
    const fixedShownItemCount = 10
    const requestTimeoutId = useRef<number>()
    const hasLatestQueryResult = useRef<boolean>(true)
    const [hasPreview, setHasPreview] = useState<boolean>(false)


    const showApp = async () => {
        await show()
        await appWindow.setFocus()
        queryBoxRef.current?.select()
    }

    const hideApp = async () => {
        await hide()
    }

    /*
        Handle Global Request
     */
    const handleRequestCallback = (message: WOXMESSAGE.WoxMessage) => {
        if (message.Method === WoxMessageRequestMethodEnum.HideApp.code) {
            hideApp()
        }
        if (message.Method === WoxMessageRequestMethodEnum.ShowApp.code) {
            showApp()
        }
        if (message.Method === WoxMessageRequestMethodEnum.ToggleApp.code) {
            appWindow.isVisible().then(visible => {
                if (visible) {
                    hideApp()
                } else {
                    showApp()
                }
            });
        }
    }

    /*
        Reset the result list
     */
    const resetResultList = () => {
        setResultList([])
        setActiveIndex(0)
        currentIndex.current = 0
    }

    /*
        Because the query callback will be called multiple times, so we need to filter the result by query text
     */
    const handleQueryCallback = (results: WOXMESSAGE.WoxMessageResponseResult[]) => {
        setHasPreview(false)
        currentResultList.current = currentResultList.current.concat(results.filter((result) => {
            if (result.AssociatedQuery === queryText.current) {
                hasLatestQueryResult.current = true
            }
            return result.AssociatedQuery === queryText.current
        })).map((result, index) => {
            if (result.Preview.PreviewType) {
                setHasPreview(true)
            } else {
                setHasPreview(false)
            }
            return Object.assign({...result, Index: index})
        })

        //sort currentResultList order by score desc
        currentResultList.current.sort((a, b) => {
            return b.Score - a.Score
        })

        resetResultList()
        setShownResultList()
    }

    /*
        Set the result list to be shown
     */
    const setShownResultList = () => {
        const rsList = currentIndex.current >= fixedShownItemCount ? currentResultList.current.slice(currentIndex.current - fixedShownItemCount + 1, currentIndex.current + 1) : currentResultList.current.slice(0, fixedShownItemCount)
        appWindow.setSize(new LogicalSize(800, 48 + 49 * rsList.length + 2)).then(_ => {
            setResultList(rsList)
        })
    }

    /*
        Deal with the active index
     */
    const dealActiveIndex = (isUp: boolean) => {
        if (isUp) {
            if (currentIndex.current > 0) {
                currentIndex.current = currentIndex.current - 1
                setActiveIndex(currentIndex.current < 0 ? 0 : Math.min(currentIndex.current, fixedShownItemCount - 1))
                setShownResultList()
            }
        } else {
            if (currentIndex.current < currentResultList.current.length - 1) {
                currentIndex.current = currentIndex.current + 1
                setActiveIndex(currentIndex.current >= fixedShownItemCount ? fixedShownItemCount - 1 : currentIndex.current)
                setShownResultList()
            }
        }
    }

    const dealWithAction = () => {
        const result = currentResultList.current.find((result) => result.Index === currentIndex.current)
        if (result) {
            result.Actions.forEach((action) => {
                if (action.IsDefault) {
                    WoxMessageHelper.getInstance().sendMessage(WoxMessageMethodEnum.ACTION.code, {
                        id: action.Id,
                    })
                }
            })
        }
    }

    const getCurrentPreviewData = () => {
        const result = currentResultList.current.find((result) => result.Index === currentIndex.current)
        if (result) {
            return result.Preview
        }
        return {PreviewType: "", PreviewData: "", PreviewProperties: {}} as WOXMESSAGE.WoxPreview
    }

    useEffect(() => {
        WoxMessageHelper.getInstance().initialRequestCallback(handleRequestCallback)
        appWindow.setFocus()
    }, []);

    return <Style onKeyDown={(event) => {
        if (event.key === "ArrowUp") {
            dealActiveIndex(true)
            event.preventDefault()
            event.stopPropagation()
        }
        if (event.key === "ArrowDown") {
            dealActiveIndex(false)
            event.preventDefault()
            event.stopPropagation()
        }
        if (event.key === "Enter") {
            dealWithAction()
            event.preventDefault()
            event.stopPropagation()
        }
        console.log(event.key)
        if (event.key === "Escape") {
            hideApp()
            event.preventDefault()
            event.stopPropagation()
        }
    }} onWheel={(event) => {
        if (event.deltaY > 0) {
            dealActiveIndex(false)
        }
        if (event.deltaY < 0) {
            dealActiveIndex(true)
        }
    }}>
        <InputGroup size={"lg"}>
            <FormControl
                id="Wox"
                ref={queryBoxRef}
                aria-label="Wox"
                autoComplete="off"
                autoCorrect="off"
                autoCapitalize="off"
                as={"input"}
                autoFocus={true}
                onChange={(e) => {
                    queryText.current = e.target.value
                    currentResultList.current = []
                    clearTimeout(requestTimeoutId.current)
                    hasLatestQueryResult.current = false
                    WoxMessageHelper.getInstance().sendQueryMessage({
                        query: queryText.current,
                        type: "text"
                    }, handleQueryCallback)
                    requestTimeoutId.current = setTimeout(() => {
                        if (!hasLatestQueryResult.current) {
                            resetResultList()
                        }
                    }, 50)
                }}
            />
            <InputGroup.Text id="inputGroup-sizing-lg" aria-describedby={"Wox"}>Wox</InputGroup.Text>
        </InputGroup>
        {resultList?.length > 0 && <div className={"wox-query-result-container"}>
            <Row>
                <Col><ListGroup className={"wox-query-result-list"}>
                    {resultList?.map((result, index) => {
                        return <ListGroup.Item
                            key={`wox-query-result-key-${index}`}
                            active={index === activeIndex}
                            onMouseOver={() => {
                                if (result.Index !== undefined) {
                                    currentIndex.current = result.Index
                                    setActiveIndex(index)
                                }
                            }}
                            onClick={() => {
                                dealWithAction()
                            }}>
                            <div className={"wox-query-result-item"}>
                                {result.Icon.ImageType === WoxImageTypeEnum.WoxImageTypeSvg.code &&
                                    <div className={"wox-query-result-image"}
                                         dangerouslySetInnerHTML={{__html: result.Icon.ImageData}}></div>}
                                {result.Icon.ImageType === WoxImageTypeEnum.WoxImageTypeUrl.code &&
                                    <Image src={result.Icon.ImageData} className={"wox-query-result-image"}/>}
                                {result.Icon.ImageType === WoxImageTypeEnum.WoxImageTypeBase64.code &&
                                    <Image src={result.Icon.ImageData} className={"wox-query-result-image"}/>}
                                <div className={"ms-2 me-auto wox-query-result-item-intro"}>
                                    <div className={"fw-bold result-item-title"}>{result.Title}</div>
                                    <div
                                        className={"fw-lighter result-item-sub-title"}>{result.Score} - {result.SubTitle}</div>
                                </div>
                            </div>
                        </ListGroup.Item>
                    })}
                </ListGroup></Col>
                {hasPreview && <Col>
                    {getCurrentPreviewData().PreviewProperties && Object.keys(getCurrentPreviewData().PreviewProperties)?.length > 0 &&
                        <div
                            className={"wox-query-result-preview"}>
                            <div className={"wox-query-result-preview-content"}>
                                {getCurrentPreviewData().PreviewType === WoxPreviewTypeEnum.WoxPreviewTypeText.code && getCurrentPreviewData().PreviewData}
                                {getCurrentPreviewData().PreviewType === WoxPreviewTypeEnum.WoxPreviewTypeImage.code &&
                                    <Image src={getCurrentPreviewData().PreviewData}
                                           className={"wox-query-result-preview-image"}/>}
                                {getCurrentPreviewData().PreviewType === WoxPreviewTypeEnum.WoxPreviewTypeImage.code &&
                                    <Markdown>{getCurrentPreviewData().PreviewData}</Markdown>}
                            </div>

                            <div className={"wox-query-result-preview-properties"}>
                                {Object.keys(getCurrentPreviewData().PreviewProperties)?.map((key) => {
                                    return <div key={`key-${key}`}
                                                className={"wox-query-result-preview-property"}>
                                        <div
                                            className={"wox-query-result-preview-property-key"}>{key}</div>
                                        <div
                                            className={"wox-query-result-preview-property-value"}>{getCurrentPreviewData().PreviewProperties[key]}</div>
                                    </div>
                                })}
                            </div>
                        </div>}

                </Col>}
            </Row>
        </div>}
    </Style>
}

const Style = styled.div`
    .wox-query-result-list {
        max-height: 490px;
        overflow-y: hidden;
    }
    .wox-query-result-item {
        display: flex;
        align-items:center;
    }
    .wox-query-result-item-intro {
        width: 0;
        flex: 1;
    }
    .result-item-title, .result-item-sub-title {
        overflow: hidden !important;
        white-space: nowrap !important;
        text-overflow: ellipsis !important;
    }
    .wox-query-result-image {
        width: 36px;
        height: 36px;
        svg {
            width: 36px !important;
            height: 36px !important;
        }
    }
    .wox-query-result-container  {
        .row, .col {
            padding: 0 !important;
            margin: 0 !important;
        }
        border-bottom: 1px solid #dee2e6;
    }
    .wox-query-result-preview {
        position: relative;
        min-height: 490px;
        border: 1px solid #dee2e6;
        border-top: 0;
        border-bottom: 0;
        padding: 10px;
        .wox-query-result-preview-content {
            max-height: 400px;
            .wox-query-result-preview-image {
                width: 100%;
                max-height: 400px;
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
                .wox-query-result-preview-property-key,.wox-query-result-preview-property-value {
                    flex: 1;
                }
            }
            
        }
    }
`

