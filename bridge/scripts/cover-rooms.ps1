param(
    [string]$BaseUrl = $(if ($env:STS2_API_BASE) { $env:STS2_API_BASE } else { "http://127.0.0.1:8080" }),
    [int]$MaxMinutes = 12,
    [string[]]$TargetRooms = @("CHEST", "REST", "SHOP"),
    [switch]$ContinueFromMenu,
    [switch]$StartNewRun
)

$ErrorActionPreference = "Stop"

function Get-BridgeState {
    return (Invoke-RestMethod -Uri "$BaseUrl/state" -TimeoutSec 5).data
}

function Invoke-BridgeAction {
    param([hashtable]$Payload)

    $json = $Payload | ConvertTo-Json
    return (Invoke-RestMethod -Method Post -Uri "$BaseUrl/action" -ContentType "application/json" -Body $json -TimeoutSec 25).data
}

function Get-NormalizedRoomKey {
    param([string]$Value)

    switch (($Value ?? '').Trim().ToUpperInvariant()) {
        'REST' { return 'REST' }
        'RESTSITE' { return 'REST' }
        'SHOP' { return 'SHOP' }
        'MERCHANT' { return 'SHOP' }
        'CHEST' { return 'CHEST' }
        'TREASURE' { return 'CHEST' }
        'TREASUREROOM' { return 'CHEST' }
        'EVENT' { return 'EVENT' }
        'QUESTION' { return 'EVENT' }
        'QUESTIONMARK' { return 'EVENT' }
        'MONSTER' { return 'MONSTER' }
        'ELITE' { return 'ELITE' }
        default { return ($Value ?? '').Trim().ToUpperInvariant() }
    }
}

function Expand-TargetRooms {
    param([string[]]$Rooms)

    return @(
        $Rooms |
            ForEach-Object { ($_ -split ',') } |
            ForEach-Object { Get-NormalizedRoomKey $_ } |
            Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
            Select-Object -Unique
    )
}

function Get-MapChoiceIndex {
    param($MapState, [hashtable]$Coverage)

    foreach ($target in $Coverage.Keys) {
        if (-not $Coverage[$target]) {
            $match = $MapState.availableNodes |
                Where-Object { (Get-NormalizedRoomKey $_.nodeType) -eq $target } |
                Select-Object -First 1
            if ($null -ne $match) {
                return [int]$match.index
            }
        }
    }

    foreach ($candidateType in @("Monster", "MONSTER", "EVENT", "QUESTION", "CHEST", "REST", "SHOP", "Elite", "ELITE")) {
        $match = $MapState.availableNodes | Where-Object { $_.nodeType -eq $candidateType } | Select-Object -First 1
        if ($null -ne $match) {
            return [int]$match.index
        }
    }

    return [int]($MapState.availableNodes | Sort-Object row, col, index | Select-Object -First 1).index
}

function Get-IncomingDamage {
    param($CombatState)

    $total = 0
    foreach ($enemy in @($CombatState.enemies)) {
        $intentDamage = @($enemy.intents | Where-Object { $_.totalDamage -ne $null } | Measure-Object -Property totalDamage -Sum).Sum
        if ($intentDamage -ne $null) {
            $total += [int]$intentDamage
            continue
        }

        $moveId = [string]$enemy.moveId
        if ($moveId -match "ATTACK|JAB|CLAW|WHIRLWIND|STRIKE|SMASH|BITE|SLAM|HIT") {
            $total += 6
        }
    }

    return $total
}

function Get-PlayableCard {
    param($CombatState)

    $playable = @($CombatState.hand | Where-Object { $_.playable })
    if ($playable.Count -eq 0) {
        return $null
    }

    $incomingDamage = Get-IncomingDamage $CombatState
    $currentBlock = if ($null -ne $CombatState.player.block) { [int]$CombatState.player.block } else { 0 }
    $needsBlock = ($incomingDamage -gt 0) -and ($currentBlock -lt $incomingDamage)
    if ($needsBlock) {
        $defensive = @(
            $playable |
                Where-Object { $_.cardId -match "DEFEND|BARRIER|SHRUG|ENTRENCH|FLAME|IRON_WAVE" } |
                Sort-Object energyCost, index
        )
        if ($defensive.Count -gt 0) {
            return $defensive[0]
        }
    }

    $targetless = @(
        $playable |
            Where-Object {
                (-not $_.requiresTarget) -and
                ($_.cardId -notmatch "DEFEND|BARRIER|SHRUG|ENTRENCH|FLAME|IRON_WAVE")
            } |
            Sort-Object energyCost, index
    )
    if ($targetless.Count -gt 0) {
        return $targetless[0]
    }

    return ($playable | Sort-Object energyCost, index | Select-Object -First 1)
}

function Get-TargetIndex {
    param($CombatState)

    $target = $CombatState.enemies |
        Where-Object { $_.isAlive -and $_.isHittable } |
        Sort-Object currentHp, index |
        Select-Object -First 1
    if ($null -eq $target) {
        return $null
    }

    return [int]$target.index
}

$coverage = [ordered]@{}
foreach ($room in (Expand-TargetRooms $TargetRooms)) {
    $coverage[$room] = $false
}

$steps = New-Object System.Collections.Generic.List[string]
$deadline = (Get-Date).AddMinutes($MaxMinutes)

while ((Get-Date) -lt $deadline) {
    $state = Get-BridgeState
    $screen = [string]$state.screen
    $actions = @($state.availableActions)
    $steps.Add("screen=$screen floor=$($state.run.floor) actions=$($actions -join ',')")

    if ($ContinueFromMenu -and $screen -eq "MAIN_MENU" -and $actions -contains "continue_run") {
        $result = Invoke-BridgeAction @{ action = "continue_run" }
        $steps.Add("  -> continue_run => $($result.state.screen)")
        Start-Sleep -Milliseconds 400
        continue
    }

    if ($StartNewRun -and $screen -eq "MAIN_MENU" -and $actions -contains "open_character_select") {
        $result = Invoke-BridgeAction @{ action = "open_character_select" }
        $steps.Add("  -> open_character_select => $($result.state.screen)")
        Start-Sleep -Milliseconds 300
        continue
    }

    if ($StartNewRun -and $screen -eq "CHARACTER_SELECT") {
        if ($actions -contains "select_character") {
            $result = Invoke-BridgeAction @{ action = "select_character"; option_index = 0 }
            $steps.Add("  -> select_character idx=0 => $($result.state.screen)")
            Start-Sleep -Milliseconds 200
            continue
        }

        if ($actions -contains "embark") {
            $result = Invoke-BridgeAction @{ action = "embark" }
            $steps.Add("  -> embark => $($result.state.screen)")
            Start-Sleep -Milliseconds 400
            continue
        }
    }

    if ($screen -eq "MODAL") {
        if ($actions -contains "confirm_modal") {
            $result = Invoke-BridgeAction @{ action = "confirm_modal" }
            $steps.Add("  -> confirm_modal => $($result.state.screen)")
            Start-Sleep -Milliseconds 250
            continue
        }

        if ($actions -contains "dismiss_modal") {
            $result = Invoke-BridgeAction @{ action = "dismiss_modal" }
            $steps.Add("  -> dismiss_modal => $($result.state.screen)")
            Start-Sleep -Milliseconds 250
            continue
        }
    }

    if ($screen -eq "COMBAT") {
        if ($actions -contains "play_card") {
            $card = Get-PlayableCard $state.combat
            if ($null -ne $card) {
                $payload = @{ action = "play_card"; card_index = [int]$card.index }
                if ($card.requiresTarget) {
                    $targetIndex = Get-TargetIndex $state.combat
                    if ($null -ne $targetIndex) {
                        $payload.target_index = $targetIndex
                    }
                }

                $result = Invoke-BridgeAction $payload
                $steps.Add("  -> play_card idx=$($payload.card_index) target=$($payload.target_index) => $($result.state.screen)")
                Start-Sleep -Milliseconds 200
                continue
            }
        }

        if ($actions -contains "end_turn") {
            $result = Invoke-BridgeAction @{ action = "end_turn" }
            $steps.Add("  -> end_turn => $($result.state.screen)")
            Start-Sleep -Milliseconds 300
            continue
        }

        Start-Sleep -Milliseconds 250
        continue
    }

    if ($screen -eq "REWARD") {
        if ($actions -contains "claim_reward") {
            $result = Invoke-BridgeAction @{ action = "claim_reward"; option_index = 0 }
            $steps.Add("  -> claim_reward => $($result.state.screen)")
            Start-Sleep -Milliseconds 200
            continue
        }

        if ($actions -contains "proceed") {
            $result = Invoke-BridgeAction @{ action = "proceed" }
            $steps.Add("  -> reward proceed => $($result.state.screen)")
            Start-Sleep -Milliseconds 300
            continue
        }

        Start-Sleep -Milliseconds 200
        continue
    }

    if ($screen -eq "CARD_SELECTION") {
        if ($actions -contains "choose_reward_card") {
            $result = Invoke-BridgeAction @{ action = "choose_reward_card"; option_index = 0 }
            $steps.Add("  -> choose_reward_card => $($result.state.screen)")
            Start-Sleep -Milliseconds 200
            continue
        }

        if ($actions -contains "select_deck_card") {
            $result = Invoke-BridgeAction @{ action = "select_deck_card"; option_index = 0 }
            $steps.Add("  -> select_deck_card => $($result.state.screen)")
            Start-Sleep -Milliseconds 200
            continue
        }

        if ($actions -contains "skip_reward_cards") {
            $result = Invoke-BridgeAction @{ action = "skip_reward_cards" }
            $steps.Add("  -> skip_reward_cards => $($result.state.screen)")
            Start-Sleep -Milliseconds 200
            continue
        }
    }

    if ($screen -eq "EVENT" -and $actions -contains "choose_event_option") {
        $result = Invoke-BridgeAction @{ action = "choose_event_option"; option_index = 0 }
        $steps.Add("  -> choose_event_option => $($result.state.screen)")
        Start-Sleep -Milliseconds 300
        continue
    }

    if ($screen -eq "CHEST") {
        $coverage["CHEST"] = $true

        if ($actions -contains "open_chest") {
            $result = Invoke-BridgeAction @{ action = "open_chest" }
            $steps.Add("  -> open_chest => $($result.state.screen)")
            Start-Sleep -Milliseconds 300
            continue
        }

        if ($actions -contains "choose_treasure_relic") {
            $result = Invoke-BridgeAction @{ action = "choose_treasure_relic"; option_index = 0 }
            $steps.Add("  -> choose_treasure_relic => $($result.state.screen)")
            Start-Sleep -Milliseconds 300
            continue
        }

        if ($actions -contains "proceed") {
            $result = Invoke-BridgeAction @{ action = "proceed" }
            $steps.Add("  -> chest proceed => $($result.state.screen)")
            Start-Sleep -Milliseconds 300
            continue
        }
    }

    if ($screen -eq "REST") {
        $coverage["REST"] = $true

        if ($actions -contains "choose_rest_option") {
            $option = $state.rest.options | Where-Object { $_.isEnabled } | Select-Object -First 1
            if ($null -ne $option) {
                $result = Invoke-BridgeAction @{ action = "choose_rest_option"; option_index = [int]$option.index }
                $steps.Add("  -> choose_rest_option idx=$($option.index) => $($result.state.screen)")
                Start-Sleep -Milliseconds 350
                continue
            }
        }

        if ($actions -contains "proceed") {
            $result = Invoke-BridgeAction @{ action = "proceed" }
            $steps.Add("  -> rest proceed => $($result.state.screen)")
            Start-Sleep -Milliseconds 300
            continue
        }
    }

    if ($screen -eq "SHOP") {
        $coverage["SHOP"] = $true

        if ($actions -contains "open_shop_inventory") {
            $result = Invoke-BridgeAction @{ action = "open_shop_inventory" }
            $steps.Add("  -> open_shop_inventory => $($result.state.screen)")
            Start-Sleep -Milliseconds 300
            continue
        }

        if ($actions -contains "buy_card") {
            $card = $state.shop.cards |
                Where-Object { $_.isStocked -and $_.enoughGold } |
                Sort-Object price, index |
                Select-Object -First 1
            if ($null -ne $card) {
                $result = Invoke-BridgeAction @{ action = "buy_card"; option_index = [int]$card.index }
                $steps.Add("  -> buy_card idx=$($card.index) => $($result.state.screen)")
                Start-Sleep -Milliseconds 300
                continue
            }
        }

        if ($actions -contains "close_shop_inventory") {
            $result = Invoke-BridgeAction @{ action = "close_shop_inventory" }
            $steps.Add("  -> close_shop_inventory => $($result.state.screen)")
            Start-Sleep -Milliseconds 300
            continue
        }

        if ($actions -contains "proceed") {
            $result = Invoke-BridgeAction @{ action = "proceed" }
            $steps.Add("  -> shop proceed => $($result.state.screen)")
            Start-Sleep -Milliseconds 300
            continue
        }
    }

    if ($screen -eq "MAP" -and $actions -contains "choose_map_node") {
        $index = Get-MapChoiceIndex $state.map $coverage
        $node = $state.map.availableNodes | Where-Object { $_.index -eq $index } | Select-Object -First 1
        $result = Invoke-BridgeAction @{ action = "choose_map_node"; option_index = $index }
        $steps.Add("  -> choose_map_node idx=$index type=$($node.nodeType) => $($result.state.screen)")
        Start-Sleep -Milliseconds 300

        if ($coverage.Values -notcontains $false) {
            break
        }

        continue
    }

    if ($screen -eq "GAME_OVER") {
        $steps.Add("  -> stopping on GAME_OVER")
        break
    }

    if ($coverage.Values -notcontains $false) {
        break
    }

    Start-Sleep -Milliseconds 300
}

[pscustomobject]@{
    coverage = $coverage
    final = (Get-BridgeState)
    steps = $steps
} | ConvertTo-Json -Depth 18
