import ReactDOM from 'react-dom/client'
import './assets/index.css'
import 'bootstrap/dist/css/bootstrap.min.css';
import {createBrowserRouter, RouterProvider} from "react-router-dom";
import {WoxMessageHelper} from "./utils/WoxMessageHelper.ts";
import React from "react";
import WoxQueryBox from "./components/WoxQueryBox.tsx";

WoxMessageHelper.getInstance().initialize("34987");

const router = createBrowserRouter([
    {
        path: "/",
        element: <WoxQueryBox/>,
    },
    {
        path: "about",
        element: <div>About</div>,
    },
]);

ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
        <RouterProvider router={router}/>
    </React.StrictMode>
)

