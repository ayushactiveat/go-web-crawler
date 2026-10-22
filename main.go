package main // executable program, not any library

import "fmt" // print to console like iostream
import "io" // to read response body
import "net/http" // to make HTTP requests

func main() {

        targetURL := "https://example.com" // target URL we want to crawl

        // makeing HTTP get request
        fmt.Println("Fetching:", targetURL)
        resp, err := http.Get(targetURL)

        // Handle any network erros( eg no internet, bad URL)
        if err != nil {
                fmt.Println("Error fetching URL:", err)
                return
        }

        // defer closing body until func ends to prevent memory loss
        defer resp.Body.Close()

        fmt.Println("Status Code:", resp.StatusCode)

        // read entire html body into a byte slice
        body, err := io.ReadAll(resp.Body)
        if err != nil {
                fmt.Println("Error reading body:", err)
                return
        }

        // convert byte slice to a string, print first 300 char
        htmlString := string(body)

        //safety check, to check if pageis longer than 300 chars
        if len(htmlString) > 300 {
                fmt.Println("\nFirst 300 chars of HTML:")
                fmt.Println(htmlString[:300])
        } else {
                fmt.Println("\nHTML:")
                fmt.Println(htmlString)
        }
}
