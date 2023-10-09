import {Form, InputGroup, ListGroup} from "react-bootstrap";
import {useRef, useState} from "react";
import {WoxMessageHelper} from "../utils/WoxMessageHelper.ts";
import {WoxMessageMethodEnum} from "../enums/WoxMessageMethodEnum.ts";


export default () => {
    const queryText = useRef<string>()
    const currentResultList = useRef<WOXMESSAGE.WoxMessageResponseResult[]>([])
    const [resultList, setResultList] = useState<WOXMESSAGE.WoxMessageResponseResult[]>([])

    return <>
        <InputGroup size={"lg"}>
            <Form.Control
                id="Wox"
                aria-label="Wox"
                onChange={(e) => {
                    setResultList([])
                    currentResultList.current = []
                    queryText.current = e.target.value
                    WoxMessageHelper.getInstance().sendMessage(WoxMessageMethodEnum.QUERY.code, {query: queryText.current}).then((result) => {
                        const messageResult = result as WOXMESSAGE.WoxMessageResponseResult[]
                        setResultList(currentResultList.current.concat(messageResult.filter((result) => result.AssociatedQuery === queryText.current)))
                    })
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