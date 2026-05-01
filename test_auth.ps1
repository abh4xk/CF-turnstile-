# Test API key authentication

echo "Testing with invalid API key..."

$body = @{
    clientKey = "INVALID_API_KEY"
    task = @{
        type = "TurnstileTask"
        websiteURL = "https://onlyfans.com/"
        websiteKey = "0x4AAAAAAAxTpmbMvo7Qj6zy"
    }
} | ConvertTo-Json -Depth 3

try {
    $response = Invoke-WebRequest -Uri "http://localhost:5073/createTask" -Method POST -ContentType "application/json" -Body $body -UseBasicParsing
    $result = $response.Content | ConvertFrom-Json
    echo "Invalid API Key Response:"
    echo $result | ConvertTo-Json -Depth 3
    
    if ($result.errorId -eq 1) {
        echo "✅ API key authentication working correctly - invalid key rejected"
    } else {
        echo "❌ API key authentication failed - invalid key was accepted"
    }
} catch {
    echo "Error occurred: $($_.Exception.Message)"
}

echo "`nTesting with valid API key..."

$body2 = @{
    clientKey = "TEST_API_KEY_12345"
    task = @{
        type = "TurnstileTask"
        websiteURL = "https://onlyfans.com/"
        websiteKey = "0x4AAAAAAAxTpmbMvo7Qj6zy"
    }
} | ConvertTo-Json -Depth 3

try {
    $response2 = Invoke-WebRequest -Uri "http://localhost:5073/createTask" -Method POST -ContentType "application/json" -Body $body2 -UseBasicParsing
    $result2 = $response2.Content | ConvertFrom-Json
    echo "Valid API Key Response:"
    echo $result2 | ConvertTo-Json -Depth 3
    
    if ($result2.errorId -eq 0) {
        echo "✅ Valid API key accepted - task created successfully"
    } else {
        echo "❌ Valid API key rejected"
    }
} catch {
    echo "Error occurred: $($_.Exception.Message)"
}
