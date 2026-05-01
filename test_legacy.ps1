# Test backward compatibility with old endpoints

echo "Testing legacy GET endpoints..."

# Test old turnstile endpoint
try {
    $response = Invoke-WebRequest -Uri "http://localhost:5073/turnstile?url=https://onlyfans.com/&sitekey=0x4AAAAAAAxTpmbMvo7Qj6zy&action=login" -UseBasicParsing
    $result = $response.Content | ConvertFrom-Json
    echo "Legacy turnstile endpoint response:"
    echo $result | ConvertTo-Json -Depth 3
    
    if ($result.errorId -eq 0) {
        $taskId = $result.taskId
        echo "✅ Legacy endpoint working - task created: $taskId"
        
        # Test old result endpoint
        Start-Sleep -Seconds 2
        
        $resultResponse = Invoke-WebRequest -Uri "http://localhost:5073/result?id=$taskId" -UseBasicParsing
        $resultData = $resultResponse.Content | ConvertFrom-Json
        
        echo "Legacy result endpoint response:"
        echo $resultData | ConvertTo-Json -Depth 3
        
        if ($resultData.status -eq "ready" -and $resultData.solution) {
            echo "✅ Legacy result endpoint working - token received"
        } else {
            echo "Legacy result status: $($resultData.status)"
        }
    } else {
        echo "❌ Legacy endpoint failed"
    }
} catch {
    echo "Error occurred: $($_.Exception.Message)"
}

echo "`nTesting API documentation endpoint..."
try {
    $docResponse = Invoke-WebRequest -Uri "http://localhost:5073/" -UseBasicParsing
    if ($docResponse.Content -like "*Turnstile*") {
        echo "✅ Documentation endpoint working"
    } else {
        echo "❌ Documentation endpoint not working"
    }
} catch {
    echo "Error accessing documentation: $($_.Exception.Message)"
}
