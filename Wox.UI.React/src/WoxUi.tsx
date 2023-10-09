import ReactDOM from 'react-dom/client'
import './assets/index.css'
import 'bootstrap/dist/css/bootstrap.min.css';
import App from "./App.tsx";
import {BrowserRouter} from "react-router-dom";

ReactDOM.createRoot(document.getElementById('root')!).render(
    <BrowserRouter>
        <App/>
    </BrowserRouter>
)

