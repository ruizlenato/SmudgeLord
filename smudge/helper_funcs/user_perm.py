from telegram import User, Chat

from smudge import SUDO_USERS
# This module has been ported from SkyleeBot


# checks whether the user is allowed to change group info
def user_can_promote(chat: Chat, user: User, bot_id: int) -> bool:
    return chat.get_member(user.id).can_promote_members and (int(user.id) in SUDO_USERS)


# checks whether the user is allowed to change group info
def user_can_ban(chat: Chat, user: User, bot_id: int) -> bool:
    return chat.get_member(user.id).can_restrict_members and (int(user.id) in SUDO_USERS)


# checks whether the user is allowed to change group info
def user_can_pin(chat: Chat, user: User, bot_id: int) -> bool:
    return chat.get_member(user.id).can_pin_messages and (int(user.id) in SUDO_USERS)


# checks whether the user is allowed to change group info
def user_can_changeinfo(chat: Chat, user: User, bot_id: int) -> bool:
    return chat.get_member(user.id).can_change_info and (int(user.id) in SUDO_USERS)


# checks whether the user is allowed to change group info
def user_can_restrict_members(chat: Chat, user: User, bot_id: int) -> bool:
    return chat.get_member(user.id).can_restrict_members and (int(user.id) in SUDO_USERS)
