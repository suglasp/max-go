// orders.go : defines wire-stable unit orders and their state values.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : enums.hpp - UnitOrderType, UnitStateType
package game

// Order is a unit's current order (what it is doing or about to do). Values are wire stable.
//
// C++ ported : enums.hpp - UnitOrderType
type Order uint8

const (
	OrderAwait          Order = 0x00
	OrderTransform      Order = 0x01
	OrderMove           Order = 0x02
	OrderFire           Order = 0x03
	OrderBuild          Order = 0x04
	OrderActivate       Order = 0x05
	OrderNewAllocate    Order = 0x06
	OrderPowerOn        Order = 0x07
	OrderPowerOff       Order = 0x08
	OrderExplode        Order = 0x09
	OrderUnload         Order = 0x0A
	OrderClear          Order = 0x0B
	OrderSentry         Order = 0x0C
	OrderLand           Order = 0x0D
	OrderTakeOff        Order = 0x0E
	OrderLoad           Order = 0x0F
	OrderIdle           Order = 0x10
	OrderRepair         Order = 0x11
	Order17             Order = 0x12 // unnamed in the original (ORDER_17)
	OrderReload         Order = 0x13
	OrderTransfer       Order = 0x14
	OrderHaltBuilding   Order = 0x15
	OrderAwaitScaling   Order = 0x16
	OrderAwaitTapePos   Order = 0x17
	OrderAwaitStealUnit Order = 0x18
	OrderAwaitDisable   Order = 0x19
	OrderDisable        Order = 0x1A
	OrderMoveToUnit     Order = 0x1B
	OrderUpgrade        Order = 0x1C
	OrderLayMine        Order = 0x1D
	OrderMoveToAttack   Order = 0x1E
	OrderHaltBuilding2  Order = 0x1F
	OrderCountMax       Order = 0x20
)

// OrderState tracks progress within an Order's state machine. Values are wire stable.
//
// C++ ported : enums.hpp - UnitOrderStateType
type OrderState uint8

const (
	OrderStateInit                OrderState = 0x00
	OrderStateExecutingOrder      OrderState = 0x01
	OrderStateReadyToExecute      OrderState = 0x02
	OrderStateStore               OrderState = 0x03
	OrderStatePrepareStore        OrderState = 0x04
	OrderStateInProgress          OrderState = 0x05
	OrderStateInTransition        OrderState = 0x06
	OrderStateIssuingPath         OrderState = 0x07
	OrderStateReadyToFire         OrderState = 0x08
	OrderStateFireInProgress      OrderState = 0x09
	OrderState10                  OrderState = 0x0A
	OrderStateBuildInProgress     OrderState = 0x0B
	OrderStatePathRequestCancel   OrderState = 0x0C
	OrderStateBuildCancel         OrderState = 0x0D
	OrderStateDestroy             OrderState = 0x0E
	OrderStateLoadingInProgress   OrderState = 0x0F
	OrderStateFinishLoading       OrderState = 0x10
	OrderStateUnloadingInProgress OrderState = 0x11
	OrderStateFinishUnloading     OrderState = 0x12
	OrderStateFinishLanding       OrderState = 0x13
	OrderStateFinishTakeOff       OrderState = 0x14
	OrderState21                  OrderState = 0x15
	OrderStateProgressTransform   OrderState = 0x16
	OrderStateFinishTransform     OrderState = 0x17
	OrderStateClearPath           OrderState = 0x18
	OrderStateSelectSite          OrderState = 0x19
	OrderStateBuildClearing       OrderState = 0x1A
	OrderStateExplode             OrderState = 0x1B
	OrderStateMoveInit            OrderState = 0x1C
	OrderStateMoveGettingPath     OrderState = 0x1D
	OrderStateBuildingReady       OrderState = 0x1E
	OrderStateUnitReady           OrderState = 0x1F
	OrderStateExpand              OrderState = 0x20
	OrderStateShrink              OrderState = 0x21
	OrderStateTapePosInit         OrderState = 0x22
	OrderStateTapePosEnter        OrderState = 0x23
	OrderStateTapePosDeinit       OrderState = 0x24
	OrderStateTapePosLeave        OrderState = 0x25
	OrderStateElevate             OrderState = 0x26
	OrderStateLower               OrderState = 0x27
	OrderStateAttackPending       OrderState = 0x28
	OrderStateAttackBeginning     OrderState = 0x29
	OrderStateNewOrder            OrderState = 0x2A
	OrderStateInactive            OrderState = 0x2B
	OrderStatePlacingMines        OrderState = 0x2C
	OrderStateRemovingMines       OrderState = 0x2D
	OrderStateBuildAbort          OrderState = 0x2E
)
