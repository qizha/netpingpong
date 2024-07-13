Here is how this program work:
1. start a http server to accept request. 
2. when the request is coming, check if an IP address or domain name is in the parameter, send a http request to this address and check whether the response code is 200. If it is 200, response status 200 to the first request.
3. If there is no IP address or domain name in the parameter, response 200 as response status directly.
   
In additional, start an individual goroutine with a timer with a cycle of 10 seconds to send request to an pre-defined address(get from the system environment variable) with the current IP address. 
If the response is 200, call the kubernetes API-server in the current K8S cluster to remove the given taint(the name of the taint can be retrived from system environment variable) for the current node(node name can be retrieved from Downward API).
If the response is not 200, record to log and wait for the next checking round.
