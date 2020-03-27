param($dataDir)

$allFiles = $dataDir | gci -Recurse -File
$connGraphs = $allFiles | ?{$_.Name -eq "dht_graphs.out"}
$rts = $allFiles | ?{$_.Name -eq "dht_rt.out"}
$errs = $allFiles | ?{$_.Name -eq "run.err"}

$ns = 1000000000

function basicStats ($values, $reverse) {
	if ($null -eq $values) {
		return [PSCustomObject]@{
			Average = 0
			Percentile95 = 0
		}
	}
	$obj = $values | measure-object -Average -Sum -Maximum -Minimum -StandardDeviation
	if ($null -eq $reverse || $false -eq $reverse) {
		$sorted = $values | Sort-Object 
	} else {
		$sorted = $values | Sort-Object -Descending
	}
	$95percentile = $sorted[[math]::Ceiling(95 / 100 * ($sorted.Count - 1))]

	if ($null -eq $95percentile) {
		return "ASDFASFASF"
	} 

	return [PSCustomObject]@{
		Average = [math]::Round([double]$obj.Average,2)
		Percentile95 = [math]::Round([double]$95percentile, 2)
	}
}

function groupStats ($metrics, $groupIndex, $reverse) {
	$fields = @{}
	$grouped = $metrics | Group-Object -Property {$_.Name.Split("|")[$groupIndex]}
	foreach ($g in $grouped) {
		$v = basicStats ($g.Group | %{$_.Value}) $reverse
		$fields.Add($g.Name, $v)
	}
	$ret = New-Object -TypeName psobject -Property $fields
	return $ret
}

function run($groupDir) {
	$groupID = $groupDir.Name

	$files =  $groupDir | gci -Recurse -File

	$queries = $files  | ?{$_.Name -eq "dht_queries.out"}
	$lookups = $files | ?{$_.Name -eq "dht_lookup.out"}
	$out = $files  | ?{$_.Name -eq "run.out"}

	$metrics = $out | Get-Content | ConvertFrom-Json | %{$_.event.metric} | ?{$_}
	$mset =

	$provs = $metrics |
	?{$_.name -and $_.name.StartsWith("time-to-provide") -and $_.value -gt 0} |
	%{$_.value/$ns}

	$findfirst = $metrics |
	?{$_.name -and $_.name.StartsWith("time-to-find-first")} |
	%{ [pscustomobject]@{ Name=$_.name; Value= $_.value/$ns; }}

	$findlast = $metrics |
	?{$_.name -and $_.name.StartsWith("time-to-find-last")} |
	%{ [pscustomobject]@{ Name=$_.name; Value= $_.value/$ns; }}

	$findall = $metrics |
	?{$_.name -and $_.name.StartsWith("time-to-find|")} |
	%{ [pscustomobject]@{ Name=$_.name; Value= $_.value/$ns; }}

	$findgood = $metrics |
	?{$_.name -and $_.name.StartsWith("time-to-find|done")} |
	%{ [pscustomobject]@{ Name=$_.name; Value= $_.value/$ns; }}

	$findfail = $metrics |
	?{$_.name -and $_.name.StartsWith("time-to-find|fail")} |
	%{ [pscustomobject]@{ Name=$_.name; Value= $_.value/$ns; }}

	$found = $metrics |
	?{$_.name -and $_.name.StartsWith("peers-found") -and $_.value -gt 0} |
	%{ [pscustomobject]@{ Name=$_.name; Value= $_.value; }}

	$missing = $metrics |
	?{$_.name -and $_.name.StartsWith("peers-missing")} |
	%{ [pscustomobject]@{ Name=$_.name; Value= $_.value; }}

	$failures = $metrics |
	?{$_.name -and $_.name.StartsWith("peers-found") -and $_.value -eq 0} |
	%{ [pscustomobject]@{ Name=$_.name; Value= $_.value; }}

	$dials = $queries | %{Get-Content $_ | ConvertFrom-Json | ?{$_.msg -eq "dialing"} | measure-object } |
	%{$_.Count} 

	$msgs = $queries | %{Get-Content $_ | ConvertFrom-Json | ?{$_.msg -eq "send"} | measure-object } |
	%{$_.Count} 

	echo "Group $groupID :"

	if ($null -ne $provs) {
		echo "Time-to-Provide"
		basicStats($provs) | Format-Table
	}

	if ($null -ne $findfirst) {
		echo "Time-to-Find-First"
		groupStats $findfirst 1 | Format-Table

		echo "Time-to-Find-Last"
		groupStats $findlast 2 | Format-Table

		if ($null -ne $findgood) {
			echo "Time-to-Find Success"
			groupStats $findgood 2 | Format-Table
		}

		if ($null -ne $findfail) {
			echo "Time-to-Find Fail"
			groupStats $findfail 2 | Format-Table

			echo "Number of Failures"
			groupStats $failures 2 | Format-Table
		}

		if (($null -ne $findgood) -and ($null -ne $findfail)) {
			echo "Time-to-Find"
			groupStats $findall 2 | Format-Table
		}

		echo "Peers Found"
		groupStats $found 2 $true | Format-Table

		echo "Peers Missing"
		groupStats $missing 2 | Format-Table
	
		#if ($failures -ne $null) {
		#	echo "Peers Failures"
		#	groupStats $failures 2 | Format-Table
		#} else {
		#	echo "No Peer Failures"
		#}
	}

	if ($dials -ne $null) {
		echo "Total number of dials"
		basicStats($dials) | Format-Table
	} else {
		echo "No DHT query dials performed"
	}

	if ($msgs -ne $null) {
		echo "Total number of messages sent"
		basicStats($msgs) | Format-Table
	} else {
		echo "No DHT query messages sent"
	}
}

function condense($fileDir) {
	Remove-Item $fileDir/lookupcmp.json -ErrorAction Ignore
	Remove-Item $fileDir/runcmp.json -ErrorAction Ignore

	$lookupOut = gci $fileDir/dht_lookups.out | gc | ConvertFrom-Json
	$start = $lookupOut | Select-Object -First 1 -ExpandProperty ts
	$lookupOut | %{
		$_.ts = ($_.ts - $start)/$ns;
		$_.node = -join $_.node[-4..-1];
		$_.target = -join $_.target[-4..-1];
		$_.eventID = -join $_.eventID[0..4];
		$_.targetKad = -join $_.targetKad[0..4];

		if ($null -ne $_.cause) {
			sliceLast $_ cause
			sliceLast $_ source
			sliceFirst $_ causeKad
			sliceFirst $_ sourceKad
			arrslice $_ heard $false
			arrslice $_ waiting $false
			arrslice $_ queried $false
			arrslice $_ unreachable $false
			arrslice $_ heardKad $true
			arrslice $_ waitingKad $true
			arrslice $_ queriedKad $true
			arrslice $_ unreachableKad $true

		}
		$_} | Select-Object -Property * -ExcludeProperty heard,waiting,queried,unreachable |
	 %{ $_ | ConvertTo-Json -Compress | Add-Content $fileDir/lookupcmp.json }

	$runOut = gci $fileDir/run.out | gc | ConvertFrom-Json
	$runOut | %{$_.ts = ($_.ts - $start)/$ns; $_} |
	%{ $_ | ConvertTo-Json -Compress | Add-Content $fileDir/runcmp.json }
}

function sliceFirst($obj, $field) {
	if ($null -eq $obj.$field) {
		return 
	}
	$obj.$field = -join $obj.$field[0..4]
}

function sliceLast($obj, $field) {
	if ($null -eq $obj.$field) {
		return 
	}
	$obj.$field = -join $obj.$field[-4..-1]
}

function arrslice($obj, $field, $first) {
	if ($null -eq $obj.$field) {
		return
	}
	if ($first) {
		$obj.$field = $obj.$field | %{-join $_[0..4]}
	} else {
		$obj.$field = $obj.$field | %{-join $_[-4..-1]}

	}
}

run $dataDir
condense $dataDir