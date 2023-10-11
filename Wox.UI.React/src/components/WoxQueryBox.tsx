import {Form, InputGroup, ListGroup} from "react-bootstrap";
import {useRef, useState} from "react";
import {WoxMessageHelper} from "../utils/WoxMessageHelper.ts";
import styled from "styled-components";


export default () => {
    const queryText = useRef<string>()
    const currentResultList = useRef<WOXMESSAGE.WoxMessageResponseResult[]>([])
    const [resultList, setResultList] = useState<WOXMESSAGE.WoxMessageResponseResult[]>([])
    const [activeIndex, setActiveIndex] = useState<number>(0)
    const currentIndex = useRef(0)
    const fixedShownItemCount = 10


    const handleQueryCallback = (results: WOXMESSAGE.WoxMessageResponseResult[]) => {
        currentResultList.current = currentResultList.current.concat(results.filter((result) => result.AssociatedQuery === queryText.current))
        setShownResultList()
    }

    const setShownResultList = () => {
        if (currentIndex.current >= fixedShownItemCount) {
            setResultList(currentResultList.current.slice(currentIndex.current - fixedShownItemCount + 1, currentIndex.current + 1))
        } else {
            setResultList(currentResultList.current.slice(0, fixedShownItemCount))
        }
    }

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
        } else if (event.key === "ArrowDown") {
            dealActiveIndex(false)
            event.preventDefault()
            event.stopPropagation()
        }
    }} onWheel={(event) => {
        console.log(event)
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
                    setResultList([])
                    currentResultList.current = []
                    queryText.current = e.target.value
                    WoxMessageHelper.getInstance().sendQueryMessage({query: queryText.current}, handleQueryCallback)
                }}
            />
            <InputGroup.Text id="inputGroup-sizing-lg" aria-describedby={"Wox"}>Wox</InputGroup.Text>
        </InputGroup>
        <ListGroup className={"wox-query-result-list"} onMouseOver={(event) => {
            console.log(event)
        }}>
            {resultList?.map((result, index) => {
                return <ListGroup.Item action eventKey={`wox-query-result-event-key-${index}`}
                                       key={`wox-query-result-key-${index}`}
                                       active={index === activeIndex}>{result.Title}</ListGroup.Item>
            })}
        </ListGroup>

    </Style>
}

const Style = styled.div`
    .wox-query-result-list {
        max-height: 500px;
        overflow-y: hidden;
    }
`

