# Test the new professional API format

# Test 1: Create Task
echo "Testing POST /createTask..."
$body = @{
    clientKey = "TEST_API_KEY_12345"
    task = @{
        type = "TurnstileTask"
        websiteURL = "https://onlyfans.com/"
        websiteKey = "0x4AAAAAAAxTpmbMvo7Qj6zy"
        action = "login"
    }
} | ConvertTo-Json -Depth 3

try {
    $response = Invoke-WebRequest -Uri "http://localhost:5073/createTask" -Method POST -ContentType "application/json" -Body $body -UseBasicParsing
    $result = $response.Content | ConvertFrom-Json
    echo "Create Task Response:"
    echo $result | ConvertTo-Json -Depth 3
    
    if ($result.errorId -eq 0) {
        $taskId = $result.taskId
        echo "Task created successfully with ID: $taskId"
        
        # Test 2: Get Task Result
        echo "`nTesting POST /getTaskResult..."
        $resultBody = @{
            clientKey = "TEST_API_KEY_12345"
            taskId = $taskId
        } | ConvertTo-Json
        
        Start-Sleep -Seconds 2  # Wait a bit for processing
        
        for ($i = 0; $i -lt 10; $i++) {
            $resultResponse = Invoke-WebRequest -Uri "http://localhost:5073/getTaskResult" -Method POST -ContentType "application/json" -Body $resultBody -UseBasicParsing
            $resultData = $resultResponse.Content | ConvertFrom-Json
            
            echo "Check $i - Status: $($resultData.status)"
            
            if ($resultData.status -eq "ready") {
                echo "Task completed successfully!"
                echo "Token: $($resultData.solution.token)"
                break
            }
            
            if ($resultData.status -ne "processing") {
                echo "Task failed or error occurred"
                break
            }
            
            Start-Sleep -Seconds 3
        }
    } else {
        echo "Task creation failed"
    }
} catch {
    echo "Error occurred: $($_.Exception.Message)"
}
