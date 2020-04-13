param($graphID,$dataDir)

$allGraphs = @("br", "bt", "at", "ab", "end")
$fmt = "png"

if (!$dataDir) {
	$dataDir = gci $HOME/.testground/results/dht | Sort LastWriteTime | select -Last 1
}

if ($graphID) {
	$allGraphs = @($graphID)
}

$allGraphs | %{
	$g = $_
	$data = gci $dataDir -recurse | ?{$_.Name -eq "stderr.json"} | Get-Content |
	ConvertFrom-Json
	
	$gdataz = $data | ?{$_.N -eq "Graph" -and $_.M -eq $g}
    $gdata = $gdataz | %{"Z{0} -> Z{1};`n" -f $_.From, $_.To}
	$file = "digraph D {`n " + $gdata + "}"
	$file > "$g-conn.dot"
	
	$rtdata = $data | ?{$_.N -eq "RT" -and $_.M -eq "$g"} | %{"Z{0} -> Z{1};`n" -f $_.From, $_.To}
	$rtfile = "digraph D {`n " + $rtdata + "}"
	$rtfile > "$g-rt.dot"
	
	#$file | circo "-T$fmt" -o "$g.$fmt"
}
