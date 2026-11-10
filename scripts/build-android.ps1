param(
    [Parameter(ValueFromRemainingArguments = $true)][string[]]$Args
)
$env:JAVA_HOME = "D:\Android Studio\jbr"
$env:ANDROID_HOME = "C:\Users\sivan\AppData\Local\Android\Sdk"
$gradle = "C:\Users\sivan\.gradle\wrapper\dists\gradle-9.5.0-bin\bvnork1r7n8i6kp5cnkibsc9q\gradle-9.5.0\bin\gradle.bat"
& $gradle -p android @Args
exit $LASTEXITCODE
