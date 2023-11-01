import "./App.css"

function App() {

  const onKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter") {
      const value = event.currentTarget.value
      if (value) {
        // send post request to server

      }
    }
  }

  return (
    <>
      <ul>
        <li></li>
      </ul>

      <div className="input-container">
        <input placeholder="please input" onKeyDown={onKeyDown} />
      </div>
    </>
  )
}

export default App
