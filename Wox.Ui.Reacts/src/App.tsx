import {Route, Routes} from "react-router";
import WoxQueryBox from "./components/WoxQueryBox.tsx";
import {useCallback, useEffect} from "react";
import useWebSocket from "react-use-websocket";
import WoxMessage = WEBSOCKET.WoxMessage;

export default () => {

    //Get websocket URL from server
    const getSocketUrl = useCallback(() => {
        return new Promise<string>((resolve) => {
            setTimeout(() => {
                resolve('ws://127.0.0.1:34987/ws');
            }, 2000);
        });
    }, []);

    const {
        lastMessage,
        sendMessage
    } = useWebSocket(getSocketUrl, {
        onOpen: () => console.log('opened'),
        //Will attempt to reconnect on all close events, such as server shutting down
        reconnectAttempts: 10,
        reconnectInterval: 2000,
        heartbeat: {
            message: 'ping',
            returnMessage: 'pong',
            timeout: 60000, // 1 minute, if no response is received, the connection will be closed
            interval: 10000, // every 10 seconds, a ping message will be sent
        },
    });


    //TODO: call this function to send a message to the server
    const sendWoxMessage = async (woxMessage: WoxMessage): Promise<unknown> => {
        sendMessage(JSON.stringify(woxMessage));
        return "111"
    }

    useEffect(() => {
        if (lastMessage !== null) {
            console.log(lastMessage)
        }
    }, [lastMessage]);

    return <>
        <Routes>
            <Route path={"/"} element={<WoxQueryBox sendWoxMessage={sendWoxMessage}/>}></Route>
        </Routes>
    </>
}