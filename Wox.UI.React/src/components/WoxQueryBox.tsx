import {Form, Image, InputGroup, ListGroup} from "react-bootstrap";
import {useRef, useState} from "react";
import {WoxMessageHelper} from "../utils/WoxMessageHelper.ts";
import styled from "styled-components";
import {WOXMESSAGE} from "../entity/WoxMessage.typings";
import {WoxMessageMethodEnum} from "../enums/WoxMessageMethodEnum.ts";

export default () => {
    const queryText = useRef<string>()
    const currentResultList = useRef<WOXMESSAGE.WoxMessageResponseResult[]>([])
    const [resultList, setResultList] = useState<WOXMESSAGE.WoxMessageResponseResult[]>([])
    const [activeIndex, setActiveIndex] = useState<number>(0)
    const currentIndex = useRef(0)
    const fixedShownItemCount = 10
    const requestTimeoutId = useRef<number>()
    const hasLatestQueryResult = useRef<boolean>(false)
    const [hasPreview, setHasPreview] = useState<boolean>(false)

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
        currentResultList.current = currentResultList.current.concat(results.filter((result) => {
            if (result.AssociatedQuery === queryText.current) {
                hasLatestQueryResult.current = true
            }
            return result.AssociatedQuery === queryText.current
        })).map((result, index) => {
            return Object.assign({...result, Index: index})
        })
        resetResultList()
        setShownResultList()
    }

    /*
        Set the result list to be shown
     */
    const setShownResultList = () => {
        if (currentIndex.current >= fixedShownItemCount) {
            setResultList(currentResultList.current.slice(currentIndex.current - fixedShownItemCount + 1, currentIndex.current + 1))
        } else {
            setResultList(currentResultList.current.slice(0, fixedShownItemCount))
        }
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
            <Form.Control
                id="Wox"
                aria-label="Wox"
                onChange={(e) => {
                    currentResultList.current = []
                    queryText.current = e.target.value
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

        <ListGroup className={"wox-query-result-list"}>
            {resultList?.map((result, index) => {
                return <ListGroup.Item
                    key={`wox-query-result-key-${index}`}
                    active={index === activeIndex} onMouseOver={() => {
                    if (result.Index !== undefined) {
                        currentIndex.current = result.Index
                        setActiveIndex(index)
                    }
                }}>
                    <div className={"wox-query-result-item"}>
                        <Image src={result.Icon.ImageData} className={"wox-query-result-image"}/>
                        <div className={"ms-2 me-auto"}>
                            <div className={"fw-bold"}>{result.Title}</div>
                            <div className={"fw-lighter"}>{result.SubTitle}</div>
                        </div>
                    </div>
                </ListGroup.Item>
            })}
        </ListGroup>

    </Style>
}

const Style = styled.div`
    .wox-query-result-list {
        max-height: 500px;
        overflow-y: hidden;
    }
    .wox-query-result-item {
        display: flex;
        align-items:center
    }
    .wox-query-result-image {
        width: 36px;
        height: 36px;
    }
`

