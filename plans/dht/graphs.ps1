param($graphID,$dataDir)

$allGraphs = @("br", "bt", "at")

if (!$dataDir) {
	$dataDir = gci $HOME/.testground/results/dht | Sort LastWriteTime | select -Last 1
}

if (!$graphID) {
	$allGraphs | %{
		$g = $_
		$links = gci $dataDir -recurse | ?{$_.Name -eq "stderr.json"} | Get-Content |
		ConvertFrom-Json | ?{$_.N -eq "Graph.$g"} | %{$_.M} | ConvertFrom-Json |
		%{$_.From+" -> "+$_.To+";"}
		
		"digraph D { `n" + $links + "`n }" | circo -Tsvg -o "$g.svg"
	}
} else {
	$g = $graphID
	$links = gci $dataDir -recurse | ?{$_.Name -eq "stderr.json"} | Get-Content |
		ConvertFrom-Json | ?{$_.N -eq "Graph.$g"} | %{$_.M} | ConvertFrom-Json |
		%{$_.From+" -> "+$_.To+";"}
		
		"digraph D { `n" + $links + "`n }" | circo -Tsvg -o "$g.svg"
}