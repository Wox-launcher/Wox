import {Form, InputGroup, ListGroup} from "react-bootstrap";
import {useRef} from "react";
import {WoxMessageHelper} from "../utils/WoxMessageHelper.ts";
import {WoxMessageMethodEnum} from "../enums/WoxMessageMethodEnum.ts";


export default () => {
    const queryText = useRef<string>()
    return <>
        <InputGroup size={"lg"}>
            <Form.Control
                id="Wox"
                aria-label="Wox"
                onChange={(e) => {
                    queryText.current = e.target.value
                    WoxMessageHelper.getInstance().sendMessage(WoxMessageMethodEnum.QUERY.code, {query: queryText.current})
                }}
            />
            <InputGroup.Text id="inputGroup-sizing-lg" aria-describedby={"Wox"}>Wox</InputGroup.Text>
        </InputGroup>
        <ListGroup>
            <ListGroup.Item>Cras justo odio</ListGroup.Item>
            <ListGroup.Item>Dapibus ac facilisis in</ListGroup.Item>
            <ListGroup.Item>Morbi leo risus</ListGroup.Item>
            <ListGroup.Item>Porta ac consectetur ac</ListGroup.Item>
            <ListGroup.Item>Vestibulum at eros</ListGroup.Item>
        </ListGroup>

    </>
}