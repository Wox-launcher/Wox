<div id="plugin-store">
    <input type="text" id="search" placeholder="Search for plugins...">
</div>

<script type="application/javascript">
fetch('https://raw.githubusercontent.com/Wox-launcher/Wox/master/store-plugin.json')
    .then(response => response.json())
    .then(data => {
        let table = document.createElement('table');
        table.style.width = '100%';

        let thead = document.createElement('thead');
        let headerRow = document.createElement('tr');
        let headers = ['Icon', 'Name', 'Description', 'Author',  'Version', 'Install'];
        headers.forEach(header => {
            let th = document.createElement('th');
            if (header === 'Icon') {
                th.style.width = '40px';
            }           
            if (header === 'Name') {
                th.style.width = '300px';
            }   
            if (header === 'Author') {
                th.style.width = '160px';
            }   
            if (header === 'Version') {
                th.style.width = '100px';
            }
            if (header === 'Description') {
                th.style.width = '500px';
            }
            th.textContent = header;
            headerRow.appendChild(th);
        });
        thead.appendChild(headerRow);
        table.appendChild(thead);

        let tbody = document.createElement('tbody');
        tbody.id = 'pluginTable'; 
        data.forEach(plugin => {
            let row = document.createElement('tr');
            let cells = [
                `<img src="${plugin.IconUrl}" width="32" height="32" style="max-width:none;">`,
               `<a href="${plugin.Website}" target="_blank">${plugin.Name}</a>`,
                plugin.Description,
                plugin.Author,
                `v${plugin.Version}`,
                `<a href="wox://query?q=wpm install ${plugin.Name}" target="_blank">Install</a>`
            ];
            cells.forEach(cell => {
                let td = document.createElement('td');
                td.innerHTML = cell;
                row.appendChild(td);
            });
            tbody.appendChild(row);
        });
        table.appendChild(tbody);

        document.getElementById('plugin-store').appendChild(table);
    });

    function searchFunction() {
        let input, filter, table, tr, td, i, txtValue;
        input = document.getElementById("search");
        filter = input.value.toUpperCase();
        table = document.getElementById("pluginTable");
        tr = table.getElementsByTagName("tr");
        for (i = 0; i < tr.length; i++) {
            td = tr[i].getElementsByTagName("td")[1];
            if (td) {
                txtValue = td.textContent || td.innerText;
                if (txtValue.toUpperCase().indexOf(filter) > -1) {
                    tr[i].style.display = "";
                } else {
                    tr[i].style.display = "none";
                }
            }
        }
    }

    document.getElementById('search').addEventListener('keyup', searchFunction);
</script>


<style>
#search {
    border: 1px solid #ccc;
    padding: 10px;
    font-size: 16px;
    border-radius: 5px;
    margin-bottom: 20px; 
}
table {
    border-collapse: collapse;
    width: 100%;
    clear: both;
}
th, td {
    border: 1px solid #ddd;
    padding: 8px;
}
tr:nth-child(even) {
    background-color: #f2f2f2;
}
th {
    background-color: #4CAF50;
    color: white;
    text-align: left;
}
</style>