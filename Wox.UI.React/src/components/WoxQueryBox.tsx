import {Form, InputGroup, ListGroup} from "react-bootstrap";
import {useRef, useState} from "react";
import {WoxMessageHelper} from "../utils/WoxMessageHelper.ts";


export default () => {
    const queryText = useRef<string>()
    const currentResultList = useRef<WOXMESSAGE.WoxMessageResponseResult[]>([])
    const [resultList, setResultList] = useState<WOXMESSAGE.WoxMessageResponseResult[]>([])

    const handleQueryCallback = (results: WOXMESSAGE.WoxMessageResponseResult[]) => {
        currentResultList.current = currentResultList.current.concat(results.filter((result) => result.AssociatedQuery === queryText.current))
        setResultList(currentResultList.current)
    }

    return <>
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
        <ListGroup>
            {resultList?.map((result) => {
                return <ListGroup.Item>{result.Title}</ListGroup.Item>
            })}
        </ListGroup>
    </>
}